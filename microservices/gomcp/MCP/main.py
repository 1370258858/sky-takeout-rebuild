# 调用创建订单
import asyncio
import sys
import os
from pathlib import Path

# 添加 pmcp 目录到 sys.path，以便导入 a 模块
sys.path.insert(0, str(Path(__file__).parent.parent / "pmcp"))

from contextlib import AsyncExitStack
from contextlib import AsyncExitStack
from inspect import stack
import json
from a import normalize_mcp_result , get_field,mcp_tool_to_openai_schema,_parse_tool_arguments,normalize_mcp_result,add_to_history
from pyexpat.errors import messages
from pyexpat.errors import messages
import re
from typing import List, Dict, Optional
from elicitation import elicitation_handler

from mcp import ClientSession,  types
from mcp.client.streamable_http import streamablehttp_client
from openai import OpenAI







# 读取json文件
with open("../pmcp/mcp_tool_routes.json", "r") as f:
    json_data = json.load(f)

services_cfg = json_data.get("services", {})

def _resolve_url(service_name: str) -> str:
    cfg = services_cfg.get(service_name, {})
    env_name = cfg.get("urlEnv")
    default_url = cfg.get("defaultUrl")
    if env_name:
        return os.getenv(env_name, default_url)
    return default_url

MCP_ORDER_SERVER_URL = _resolve_url("order")

# llm config
MODEL_NAME = os.getenv("MODEL_NAME", "deepseek-v4-pro")
MODEL_MAX_HISTORY = os.getenv("MODEL_MAX_HISTORY", "10")
llm = OpenAI(api_key="sk-3dad08434cf2403199dce62cd7c1b972",base_url="https://dashscope.aliyuncs.com/compatible-mode/v1")


SYSTEM_PROMPT = (
    "你是订单助手。若需要下单请调用 create_order 工具。"
    "若需要查看购物车请调用 cart_detail 工具。"
    "若需要修改购物车请调用 update_cart 工具。"
    "若需要删除购物车请调用 delete_cart 工具。"
    "若需要查看商品列表请调用 list_goods 工具。"
    "创建订单时禁止臆造 userId。若用户未提供 userId，请在 create_order 参数中传 userId=0，由 MCP 服务端通过 Elicit 继续补充。"
    "工具返回后再给最终中文答复。"
)




# agent call  return should be total call need some thing todo
def agent_call(history, user_query=None, tools=None, tool_choice=None) -> any:
    message_content = [{"role": "system", "content": SYSTEM_PROMPT}, *history]
    if user_query is not None:
        message_content.append({"role": "user", "content": user_query})

    req = {
        "model": MODEL_NAME,
        "messages": message_content,
        "temperature": 0,
    }
    if tools:
        req["tools"] = tools
    if tool_choice is not None:
        req["tool_choice"] = tool_choice

    resp = llm.chat.completions.create(**req)
    return resp


async def main():
    history : List[Dict] = []


    async with AsyncExitStack() as stack:
        order_read, order_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_ORDER_SERVER_URL))
        order_session = await stack.enter_async_context(
            ClientSession(order_read, order_write, elicitation_callback=elicitation_handler)
        )
        await order_session.initialize()

        order_tools_result = await order_session.list_tools()
        order_tools = getattr(order_tools_result, "tools", [])
        # 转换 MCP 工具为 OpenAI 兼容格式
        llm_tools = [mcp_tool_to_openai_schema(t) for t in order_tools]

        while True:
            query = input("我是一个AI助手,有什么需求吗？")
            if not query:
                break
            history.append({"role": "user", "content": query})
            forced_tool_choice = None
           
            resp = agent_call(history=history, tools=llm_tools, tool_choice=forced_tool_choice)

            choice = resp.choices[0] if resp.choices else None
            # 取一个可能不存在属性
            if choice and choice.message:
                tool_calls = choice.message.tool_calls
            else:
                tool_calls = None

            if tool_calls:
                 # 将 assistant 的 tool_calls 回填到历史中，供下一轮消费 tool 结果
                 add_to_history(history, choice.message, tool_calls)
                 #   调用mcp方法
                 for tc in tool_calls:
                    name = tc.function.name
                    args = _parse_tool_arguments(tc.function.arguments)
                    tool_result = await order_session.call_tool(name, arguments=args)
                    payload = normalize_mcp_result(tool_result)
                                    # 将工具执行结果转为 JSON 字符串，回填给 LLM
                    content_text = json.dumps(payload, ensure_ascii=False)

                    history.append({
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": content_text,
                })
            # 根据调用结果生成回复
            resp = agent_call(history=history, tools=llm_tools)
            # 取一个可能不存在属性
            choice = resp.choices[0] if resp.choices else None
            if choice and choice.message:
                history.append({"role": "assistant", "content": choice.message.content or ""})
                print(choice.message.content)




if __name__ == "__main__":
    asyncio.run(main())

    # "帮我创建一个订单：userId=1, goodId=51, amount=56, addressBookId=1"