# agent 实现
import json
import sys
from pathlib import Path
from typing import Dict, List, Optional

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
from config.config import MODEL_NAME, SYSTEM_PROMPT, llm, MODEL_MAX_HISTORY
from core.memory import ConversationMemory
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
        self._sessions: Dict[str, ClientSession] = {}
        self._llm_tools: List[Dict] = []
        self._exit_stack: Optional[AsyncExitStack] = None

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
        return llm.chat.completions.create(**req)

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
        try:
            while True:
                query = input(f"\n当前会话id：{session_id}\n我是一个AI助手,有什么需求吗？").strip()
                if not query:
                    break

                self.memory.add("user", query)

                # 第一轮：LLM 判断是否调用工具
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
                        tool_result = await self.call_tool_by_name(tc.function.name, args)
                        payload = normalize_mcp_result(tool_result)
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
