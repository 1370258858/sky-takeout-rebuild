# agent 实现
import json
import logging
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

_MCP_ROOT = Path(__file__).parent.parent
sys.path.insert(0, str(_MCP_ROOT))

from contextlib import AsyncExitStack

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from config.config import get_seesion

from core.unit import (
    normalize_mcp_result,
    mcp_tool_to_openai_schema,
    _parse_tool_arguments,
    add_to_history,
)
from config.config import MODEL_NAME, GET_INTENT_MODEL_NAME,SYSTEM_PROMPT, GET_INTENT_PROMPT,llm, MODEL_MAX_HISTORY
from core.memory import ConversationMemory, FactMemory
from elicitation import elicitation_handler


class AgentLoop:
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

    def _log_llm_call(self, messages: List[Dict], req: Dict, resp: Any = None, error: Optional[Exception] = None) -> None:
        """记录每次 LLM 调用的上下文、token 用量与响应体。"""
        entry = {
            "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "kind": "llm",
            "model": req.get("model"),
            "tool_choice": req.get("tool_choice"),
            "context_message_count": len(messages),
            "context_messages": messages,
            "response": self._serialize_response(resp) if resp is not None else None,
            "error": str(error) if error is not None else None,
        }
        self._llm_logger.info(json.dumps(entry, ensure_ascii=False))

    def _log_tool_call(
        self,
        tool_name: str,
        args: Dict[str, Any],
        tool_call_id: Optional[str] = None,
        result: Any = None,
        error: Optional[Exception] = None,
    ) -> None:
        """记录每次 tool 调用的入参与结果。"""
        entry = {
            "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "kind": "tool",
            "tool_name": tool_name,
            "tool_call_id": tool_call_id,
            "args": args,
            "result": self._serialize_response(result) if result is not None else None,
            "error": str(error) if error is not None else None,
        }
        self._llm_logger.info(json.dumps(entry, ensure_ascii=False))


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
        req: Dict = {
            "model": GET_INTENT_MODEL_NAME,
            "messages": messages,
            "temperature": 0,
        }
        resp = llm.chat.completions.create(**req)
        self._log_llm_call(messages=messages, req=req, resp=resp)
        return resp

    def _extract_json_object(self, text: str) -> Optional[Dict[str, Any]]:
        """从模型返回文本中提取第一个 JSON 对象。"""
        if not text:
            return None
        text = text.strip()
        try:
            parsed = json.loads(text)
            return parsed if isinstance(parsed, dict) else None
        except json.JSONDecodeError:
            pass

        start = text.find("{")
        end = text.rfind("}")
        if start == -1 or end == -1 or end <= start:
            return None
        try:
            parsed = json.loads(text[start : end + 1])
            return parsed if isinstance(parsed, dict) else None
        except json.JSONDecodeError:
            return None

    def _apply_budget_intent_result(self, result_text: str) -> bool:
        """解析意图模型返回并在可用时写入预算事实。"""
        payload = self._extract_json_object(result_text)
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
    def _llm_call(self, tool_choice=None):
        """将当前内存中的历史与 system prompt 一起发给 LLM。"""
        messages = [
            {"role": "system", "content": SYSTEM_PROMPT},
            *self.memory.get_history(),
        ]
        req: Dict = {
            "model": MODEL_NAME,
            "messages": messages,
            "temperature": 0,
            "tools": self._llm_tools,
        }
        if tool_choice is not None:
            req["tool_choice"] = tool_choice
        try:
            resp = llm.chat.completions.create(**req)
            self._log_llm_call(messages=messages, req=req, resp=resp)
            return resp
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
        self.factMemory = FactMemory(user_id="u456")
        turn = 0
        try:
            while True:
                turn = turn + 1
                query = input(f"\n当前会话id：{session_id}\n我是一个AI助手,有什么需求吗？").strip()
                if turn == 1:
                    # 首轮对话添加已知事实
                    fact_snapshot = self.factMemory.get_user()  # dict
                    fact_text = (
                    "FACTS_SNAPSHOT:\n"
                    + json.dumps(fact_snapshot, ensure_ascii=False, indent=2))
                    self.memory.add("system", fact_text)

                # 添加事实
                factresult =  self.factMemory.set_facts_from_user_input(query)
                # 如果没提取到预算事实,但是提取到了预算意图词则调用llm提取
                if not factresult[0] and  (self.factMemory.has_budget_intent(query)):
                    # 调用llm提取预算并判断返回结果后写入事实
                    print("检测到预算意图，调用LLM提取预算信息...")
                    intentresp = self._llm_get_intent_call(query)
                    intent_content = ""
                    if intentresp.choices and intentresp.choices[0].message:
                        intent_content = intentresp.choices[0].message.content or ""

                    if self._apply_budget_intent_result(intent_content):
                        print("预算意图提取成功，已写入预算事实。")
                    else:
                        print("预算意图提取未得到可用结构化结果。")

                # 第一轮：LLM 判断是否调用工具
                self.memory.add("user", query)
                resp = self._llm_call()
                choice = resp.choices[0] if resp.choices else None
                tool_calls = (
                    choice.message.tool_calls
                    if choice and choice.message
                    else None
                ) 

                if tool_calls:
                    # 将 assistant tool_calls 迎入历史
                    add_to_history(
                        self.memory.get_history(), choice.message, tool_calls
                    )
                    # 执行每个工具调用
                    for tc in tool_calls:
                        args = _parse_tool_arguments(tc.function.arguments)
                        try:
                            tool_result = await self.call_tool_by_name(tc.function.name, args)
                            self._log_tool_call(
                                tool_name=tc.function.name,
                                tool_call_id=tc.id,
                                args=args,
                                result=tool_result,
                            )
                        except Exception as e:
                            self._log_tool_call(
                                tool_name=tc.function.name,
                                tool_call_id=tc.id,
                                args=args,
                                error=e,
                            )
                            raise
                        payload = normalize_mcp_result(tool_result)
                        # 是否需要更新事实，检测mcp 返回结果，如果含有待更新事实则更新agent侧的facts
                        if "updateFacts" in payload:
                            self.factMemory.set_facts(payload["updateFacts"])
                        self.memory.add_raw(
                            {
                                "role": "tool",
                                "tool_call_id": tc.id,
                                "content": json.dumps(payload, ensure_ascii=False),
                            }
                        )

                    # 根据工具结果再次问 LLM
                    resp = self._llm_call()
                    choice = resp.choices[0] if resp.choices else None

                # 输出最终回复
                if choice and choice.message:
                    reply = choice.message.content or ""
                    self.memory.add("assistant", reply)
                    print(reply)

        finally:
            if self._exit_stack:
                self.memory.save_history("order")
                await self._exit_stack.__aexit__(None, None, None)
