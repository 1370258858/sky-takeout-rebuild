import os
import json
import asyncio
from openai import OpenAI

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

MCP_SERVER_URL = os.getenv("MCP_SERVER_URL", "http://127.0.0.1:8001/mcp")
MODEL = os.getenv("LLM_MODEL","deepseek-v4-pro")

llm = OpenAI( api_key="sk-3dad08434cf2403199dce62cd7c1b972",
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",)


def _get_field(obj, *names, default=None):
    for n in names:
        if isinstance(obj, dict) and n in obj:
            return obj[n]
        if hasattr(obj, n):
            return getattr(obj, n)
    return default

def mcp_tool_to_openai_schema(tool):
    name = _get_field(tool, "name")
    description = _get_field(tool, "description", default="") or ""
    input_schema = _get_field(
        tool, "inputSchema", "input_schema",
        default={"type": "object", "properties": {}}
    )
    return {
        "type": "function",
        "function": {
            "name": name,
            "description": description,
            "parameters": input_schema,
        },
    }

# 帮我单一个鸡排订单，要有两块鸡排，一瓶可乐，
# 1.mcp提供商品总列表（包括库存余量,也就是检查鸡排库存大于2，可乐大于1)，session 获得了这个mcp接口， llm填充参数，下单
# 2.返回结果
# - 把mcp 协议商品总列表 暴露出来，定义请求参数(可复用) 
# - llm 判断库存是否够，够的话 ，这个应该通过提示词告诉llm，下单前的检查步骤
# - 调用下单mcp

async def run_agent_once(user_query: str):
    # 1) 连接 Go MCP Server (streamable-http)
    async with streamablehttp_client(MCP_SERVER_URL) as (read_stream, write_stream, _):
        async with ClientSession(read_stream, write_stream) as session:
            await session.initialize()

            # 2) 动态读取 MCP 工具清单 -> 转成 LLM tools schema
            tools_result = await session.list_tools()
            tools = _get_field(tools_result, "tools", default=tools_result)
            llm_tools = [mcp_tool_to_openai_schema(t) for t in tools]

            messages = [
                {
                    "role": "system",
                    "content": (
                        "你是订单助手。若需要下单请调用 create_order 工具。"
                        "工具返回后再给最终中文答复。"
                    ),
                },
                {"role": "user", "content": user_query},
            ]

            # 3) 先让 LLM 决定要不要调工具
            resp = llm.chat.completions.create(
                model=MODEL,
                messages=messages,
                tools=llm_tools,
                temperature=0
            )
            msg = resp.choices[0].message
            tool_calls = msg.tool_calls or []

            # 4) 若 LLM 发起工具调用 -> 调 Go MCP
            if tool_calls:
                messages.append({
                    "role": "assistant",
                    "content": msg.content or "",
                    "tool_calls": [
                        {
                            "id": tc.id,
                            "type": "function",
                            "function": {
                                "name": tc.function.name,
                                "arguments": tc.function.arguments,
                            },
                        }
                        for tc in tool_calls
                    ],
                })

                for tc in tool_calls:
                    name = tc.function.name
                    args = json.loads(tc.function.arguments or "{}")
                    tool_result = await session.call_tool(name, arguments=args)

                    # MCP 返回内容转成字符串回填给 LLM
                    content_text = json.dumps(_get_field(tool_result, "structuredContent", "structured_content", default=tool_result), ensure_ascii=False)

                    messages.append({
                        "role": "tool",
                        "tool_call_id": tc.id,
                        "content": content_text,
                    })

                # 5) 把工具结果喂回 LLM，让它生成最终答复
                final_resp = llm.chat.completions.create(
                    model=MODEL,
                    messages=messages,
                    temperature=0
                )
                return final_resp.choices[0].message.content

            # 没有工具调用就直接回答
            return msg.content or "未生成回答"

if __name__ == "__main__":
    query = "帮我创建一个订单：userId=1, goodId=51, amount=56, addressBookId=1"
    print(asyncio.run(run_agent_once(query)))