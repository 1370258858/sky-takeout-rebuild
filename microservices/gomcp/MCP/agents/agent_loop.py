# agent 实现
import json
import logging
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional
from uuid import uuid4
from contextlib import AsyncExitStack

from agents.graph_workflow import AgentGraphState, GraphWorkflowMixin
from jsonschema import Draft202012Validator, ValidationError


_MCP_ROOT = Path(__file__).parent.parent
sys.path.insert(0, str(_MCP_ROOT))

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from config.config import get_seesion
from logs.logger import Logger
from request import IntentRequestBuilder , get_intent_request,get_request

from core.unit import (
    mcp_tool_to_openai_schema,
)
from config.config import MODEL_NAME, GET_INTENT_MODEL_NAME,SYSTEM_PROMPT, GET_INTENT_PROMPT,llm, MODEL_MAX_HISTORY
from core.memory import ConversationMemory, FactMemory
from elicitation import elicitation_handler



class AgentLoop(Logger, GraphWorkflowMixin):
    """
    封装了与多个 MCP 服务的会话初始化、工具加载、LLM 调用和工具分发逻辑。
    """

    def __init__(self, service_urls: Dict[str, str]):
        """
        :param service_urls: {service_name: mcp_url} 映射表
        """
        self.service_urls = service_urls
        self.memory: Optional[ConversationMemory] = None
        self.factMemory: Optional[FactMemory] = None
        self._sessions: Dict[str, ClientSession] = {}
        self._llm_tools: List[Dict] = []
        self._exit_stack: Optional[AsyncExitStack] = None
        self._llm_logger = self._init_llm_logger()
        self._obs_context: Dict[str, Any] = {
            "session_id": None,
            "turn": 0,
            "trace_id": "",
            "node": "",
        }
        self._graph = self._build_graph()

    

    def _init_llm_logger(self) -> logging.Logger:
        """初始化 LLM 调用日志，输出到 logs/llm_calls.log。"""
        logger = logging.getLogger("mcp.llm.calls")
        if logger.handlers:
            return logger

        log_dir = _MCP_ROOT / "logs"
        log_dir.mkdir(parents=True, exist_ok=True)
        log_file = log_dir / "llm_calls.log"

        handler = logging.FileHandler(log_file, encoding="utf-8")
        handler.setFormatter(logging.Formatter("%(message)s"))
        logger.setLevel(logging.INFO)
        logger.addHandler(handler)
        logger.propagate = False
        return logger

    def _serialize_response(self, resp: Any) -> Any:
        """将 SDK 响应转为可 JSON 序列化结构。"""
        if hasattr(resp, "model_dump"):
            return resp.model_dump()
        if hasattr(resp, "to_dict"):
            return resp.to_dict()
        try:
            return json.loads(str(resp))
        except (TypeError, json.JSONDecodeError):
            return str(resp)


    # ------------------------------------------------------------------
    # 初始化 session
    # ------------------------------------------------------------------
    async def init_sessions(self) -> None:
        """初始化所有 MCP service 的 session，资源由 _exit_stack 统一清理。"""
        self._exit_stack = AsyncExitStack()
        await self._exit_stack.__aenter__()

        for name, url in self.service_urls.items():
            read, write, _ = await self._exit_stack.enter_async_context(
                streamablehttp_client(url)
            )
            session = await self._exit_stack.enter_async_context(
                ClientSession(read, write, elicitation_callback=elicitation_handler)
            )
            await session.initialize()
            self._sessions[name] = session

    # ------------------------------------------------------------------
    # 获取工具列表
    # ------------------------------------------------------------------
    async def get_tools(self) -> List[Dict]:
        """拉取所有 session 的工具并转换为 OpenAI 兼容格式，缓存到 self._llm_tools。"""
        all_tools: List[Dict] = []
        for session in self._sessions.values():
            result = await session.list_tools()
            raw_tools = getattr(result, "tools", [])
            all_tools.extend(mcp_tool_to_openai_schema(t) for t in raw_tools)
        self._llm_tools = all_tools
        return all_tools
    
    
    # 获取预算意图
    def _llm_get_intent_call(self, query: str):
        """调用 LLM 来获取用户输入中的意图。"""
        messages = [
            {"role": "system", "content": GET_INTENT_PROMPT},
            {"role": "user", "content": query}
        ]
        req = get_intent_request(messages)
        resp = llm.chat.completions.create(**req)
        self._log_llm_call(messages=messages, req=req, resp=resp)

        return None if not self._validate_response(resp, get_intent_request) else resp
    

    
    def _extract_payload_from_resp(self, resp: Any) -> Optional[Dict[str, Any]]:
        """从 LLM 响应中提取有效载荷。
            当前支持从 OpenAI API 响应中提取有效载荷。
            当前只能校验resp JSON 格式是否正确。不能做简单JSON 格式修复
        
        """
        content: Any = None
        if isinstance(resp, dict):
            content = resp
        elif isinstance(resp, str):
            content = resp
        else:
            try:
                choice = resp.choices[0] if getattr(resp, "choices", None) else None
                msg = choice.message if choice else None
                content = msg.content if msg else None
            except Exception:
                return None

        if not content:
            return None

        if isinstance(content, dict):
            return content

        if isinstance(content, str):
            # 先整体解析
            try:
                data = json.loads(content)
                return data if isinstance(data, dict) else None
            except json.JSONDecodeError:
                pass

            # 再尝试截取首尾 JSON 对象
            start = content.find("{")
            end = content.rfind("}")
            if start >= 0 and end > start:
                try:
                    data = json.loads(content[start:end + 1])
                    return data if isinstance(data, dict) else None
                except json.JSONDecodeError:
                    return None

        return None


    def _validate_response(
            self,
            resp: Any,
            request_builder: IntentRequestBuilder) -> bool:
        """按照 request_builder 的 response_format.json_schema.schema 校验响应。"""
        """"校验 LLM 响应是否符合预期的 JSON Schema。"""
        payload = self._extract_payload_from_resp(resp)
        if not payload:
            return False

        # 从你已有的请求模板中拿 schema
        req = request_builder([{"role": "system", "content": GET_INTENT_PROMPT}])
        schema = (
            req.get("response_format", {})
            .get("json_schema", {})
            .get("schema")
        )
        if not isinstance(schema, dict):
            return False

        try:
            Draft202012Validator(schema).validate(payload)
            return True
        except ValidationError as e:
            self._llm_logger.info(
                json.dumps(
                    {
                        "kind": "response_validate",
                        "status": "invalid",
                        "error": e.message,
                        "path": list(e.path),
                    },
                    ensure_ascii=False,
                )
            )
            return False


    def _apply_budget_intent_result(self, result_text: str) -> bool:
        """解析意图模型返回并在可用时写入预算事实。"""
        payload = self._extract_payload_from_resp(result_text)
        if not payload:
            return False

        if bool(payload.get("has_budget_intent")) == False:
            return False

        wrote = False
        budget_max = payload.get("budget_max")
        if isinstance(budget_max, (int, float)):
            self.factMemory.set_fact(
                "budget.max",
                budget_max,
                confidence="MEDIUM",
                source="llm_intent",
            )
            wrote = True

        budget_range = payload.get("budget_range")
        if isinstance(budget_range, dict):
            min_v = budget_range.get("min")
            max_v = budget_range.get("max")
            if isinstance(min_v, (int, float)) and isinstance(max_v, (int, float)):
                if min_v > max_v:
                    min_v, max_v = max_v, min_v
                self.factMemory.set_fact(
                    "budget.min",
                    min_v,
                    confidence="MEDIUM",
                    source="llm_intent",
                )
                self.factMemory.set_fact(
                    "budget.max",
                    max_v,
                    confidence="MEDIUM",
                    source="llm_intent",
                )
                wrote = True

        return wrote
    # ------------------------------------------------------------------
    # LLM 调用
    # ------------------------------------------------------------------
    def _llm_call(self, tool_choice=None, extra_messages: Optional[List[Dict[str, Any]]] = None):
        """将当前内存中的历史与 system prompt 一起发给 LLM。"""
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT},
            *self.memory.get_history(),
        ]
        if extra_messages:
            messages.extend(extra_messages)
        req = get_request(messages=messages)
        if tool_choice is not None:
            req["tool_choice"] = tool_choice
        try:
            resp = llm.chat.completions.create(**req)
            self._log_llm_call(messages=messages, req=req, resp=resp)
            return None if not self._validate_response(resp, get_request) else resp
        except Exception as e:
            self._log_llm_call(messages=messages, req=req, error=e)
            raise

    # ------------------------------------------------------------------
    # 工具分发
    # ------------------------------------------------------------------
    async def call_tool_by_name(self, name: str, args: Dict):
        """在所有 session 中找到对应工具并调用。"""
        for session in self._sessions.values():
            tools_result = await session.list_tools()
            tool_names = {t.name for t in getattr(tools_result, "tools", [])}
            if name in tool_names:
                return await session.call_tool(name, arguments=args)
        raise ValueError(f"Tool '{name}' not found in any session")

    # ------------------------------------------------------------------
    # 主交互循环
    # 当前循环只运行一次调用tool call，tc由于大模型幻觉tc可能多次调用，未做单次调用的限制 
    # 模型如果需要处理一个串行任务(上个call 的输出作为下个call的输入)， 这个功能需要加
    # add  tc 单次限制  模型调用串行能力
    # 
    # ------------------------------------------------------------------
    async def run(self,session_id: str) -> None:
        """启动交互循环：初始化 session、加载工具、多轮对话。"""
        await self.init_sessions()
        await self.get_tools()
        # 默认第一个session
        if session_id is  None:
            session_list = get_seesion("order")
            if  len(session_list) != 0:
                session_id = session_list[0]["sessionId"]
                print(f"使用默认会话: {session_id}")

        else :session_id = session_id
        self.memory = ConversationMemory(session_id=session_id, max_history=MODEL_MAX_HISTORY)
        self.factMemory = FactMemory(user_id=456)
        turn = 0
        order_state = self.factMemory.get_fact("order.state", "Draft")
        try:
            while True:
                turn = turn + 1
                query = input(f"\n当前会话id：{session_id}\n我是一个AI助手,有什么需求吗？\n 老样子来一单请输入1").strip()
                trace_id = f"{session_id}-t{turn}-{uuid4().hex[:8]}"
                self._set_obs_context(session_id=str(session_id), turn=turn, trace_id=trace_id, node="run")
                if turn == 1:
                    # 首轮对话添加已知事实
                    fact_snapshot = self.factMemory.get_user()  # dict
                    fact_text = (
                    "FACTS_SNAPSHOT:\n"
                    + json.dumps(fact_snapshot, ensure_ascii=False, indent=2))
                    self.memory.add("system", fact_text)
                state: AgentGraphState = {
                    "query": query,
                    "turn": turn,
                    "order_state": order_state,
                    "tool_calls": [],
                    "last_tool_names": [],
                    "tool_payloads": [],
                    "runtime_tool_messages": [],
                    "reply": "",
                    "event": "NO_OP",
                    "should_exit": False,
                }
                result = await self._graph.ainvoke(state)
                self.memory.save_history("order")
                self._log_op(op="session_persist", output_summary={"history_count": len(self.memory)})

                order_state = result.get("order_state", order_state)
                reply = result.get("reply", "")
                if reply:
                    print(reply)

                if result.get("should_exit", False):
                    print(f"订单流程已结束，终态: {order_state}")
                    break

        finally:
            if self._exit_stack:
                self.memory.save_history("order")
                await self._exit_stack.__aexit__(None, None, None)
