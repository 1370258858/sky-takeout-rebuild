import os
import json
import asyncio
from contextlib import AsyncExitStack
from typing import List, Dict, Optional
from openai import OpenAI

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from a import normalize_mcp_result , get_field
# 读取json文件
with open("./mcp_tool_routes.json", "r") as f:
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
MCP_GOODS_SERVER_URL = _resolve_url("goods")
MCP_DELIVERY_SERVER_URL = _resolve_url("delivery")

MODEL = os.getenv("LLM_MODEL","deepseek-v4-pro")
MAX_HISTORY_MESSAGES = int(os.getenv("MAX_HISTORY_MESSAGES", "30"))

llm = OpenAI( api_key="sk-3dad08434cf2403199dce62cd7c1b972",
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",)

SYSTEM_PROMPT = (
    "你是订单助手。若需要下单请调用 create_order 工具。"
    "若需要查看购物车请调用 cart_detail 工具。"
    "若需要修改购物车请调用 update_cart 工具。"
    "若需要删除购物车请调用 delete_cart 工具。"
    "若需要查看商品列表请调用 list_goods 工具。"
    "工具返回后再给最终中文答复。"
)




def mcp_tool_to_openai_schema(tool):
    name = get_field(tool, "name")
    description = get_field(tool, "description", default="") or ""
    input_schema = get_field(
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



def _tool_result_to_payload(tool_result):
    # Normalize tool result so it can be safely sent back to the model.
    content = get_field(tool_result, "content", default=[])
    if not content:
        return {"ok": True, "content": []}

    payload = []
    for c in content:
        text = get_field(c, "text")
        ctype = get_field(c, "type", default="text")
        if text is not None:
            payload.append({"type": ctype, "text": text})
    return {"ok": True, "content": payload}

def _parse_tool_arguments(raw_args):
    # MCP tool arguments must be a JSON object; coerce invalid shapes to {}.
    if raw_args is None:
        return {}
    if isinstance(raw_args, dict):
        return raw_args
    if not isinstance(raw_args, str):
        return {}

    try:
        parsed = json.loads(raw_args)
    except Exception:
        return {}

    return parsed if isinstance(parsed, dict) else {}


def _trim_history(history: List[Dict], max_messages: int) -> List[Dict]:
    if len(history) <= max_messages:
        return history
    return history[-max_messages:]

# 帮我单一个鸡排订单，要有两块鸡排，一瓶可乐，
# 1.mcp提供商品总列表（包括库存余量,也就是检查鸡排库存大于2，可乐大于1)，session 获得了这个mcp接口， llm填充参数，下单
# 2.返回结果
# - 把mcp 协议商品总列表 暴露出来，定义请求参数(可复用) 
# - llm 判断库存是否够，够的话 ，这个应该通过提示词告诉llm，下单前的检查步骤
# - 调用下单mcp

async def run_agent_once(user_query: str, history: Optional[List[Dict]] = None):
    if history is None:
        history = []

    # 1) 同时连接多个 MCP 服务，并保持 session 生命周期覆盖整个函数逻辑。
    async with AsyncExitStack() as stack:
        order_read, order_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_ORDER_SERVER_URL))
        goods_read, goods_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_GOODS_SERVER_URL))
        delivery_read, delivery_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_DELIVERY_SERVER_URL))

        order_session = await stack.enter_async_context(ClientSession(order_read, order_write))
        goods_session = await stack.enter_async_context(ClientSession(goods_read, goods_write))
        delivery_session = await stack.enter_async_context(ClientSession(delivery_read, delivery_write))

        await order_session.initialize()
        await goods_session.initialize()
        await delivery_session.initialize()

        # 2) 汇总工具，并记录每个工具对应的 service session。
        tool_to_session = {}
        tools = []
        for session in (order_session, goods_session, delivery_session):
            result = await session.list_tools()
            service_tools = get_field(result, "tools", default=result) or []
            for t in service_tools:
                name = get_field(t, "name")
                if name and name not in tool_to_session:
                    tool_to_session[name] = session
                    tools.append(t)

        llm_tools = [mcp_tool_to_openai_schema(t) for t in tools]
        print(f"工具清单：{llm_tools} \n")

        messages = [{"role": "system", "content": SYSTEM_PROMPT}, *history, {"role": "user", "content": user_query}]

        # 3) 先让 LLM 决定要不要调工具。
        resp = llm.chat.completions.create(
            model=MODEL,
            messages=messages,
            tools=llm_tools,
            temperature=0,
        )
        msg = resp.choices[0].message
        print("LLM 回复：{}", msg.content or "", "工具调用：", msg.tool_calls or [])
        tool_calls = msg.tool_calls or []

        # 4) 若 LLM 发起工具调用 -> 路由到对应 MCP session。
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
                args = _parse_tool_arguments(tc.function.arguments)

                target_session = tool_to_session.get(name, order_session)
                try:
                    tool_result = await target_session.call_tool(name, arguments=args)
                    # MCP 返回内容转成字符串回填给 LLM。
                    payload = normalize_mcp_result(tool_result)
                except Exception as e:
                    payload = {
                        "ok": False,
                        "error": f"tool {name} failed: {e}",
                        "arguments": args,
                    }

                content_text = json.dumps(payload, ensure_ascii=False)

                messages.append({
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": content_text,
                })

            # 5) 把工具结果喂回 LLM，让它生成最终答复。
            final_resp = llm.chat.completions.create(
                model=MODEL,
                messages=messages,
                temperature=0,
            )
            final_text = final_resp.choices[0].message.content or "未生成回答"
            history.append({"role": "user", "content": user_query})
            history.append({"role": "assistant", "content": final_text})
            history[:] = _trim_history(history, MAX_HISTORY_MESSAGES)
            return final_text

        # 没有工具调用就直接回答。
        answer = msg.content or "未生成回答"
        history.append({"role": "user", "content": user_query})
        history.append({"role": "assistant", "content": answer})
        history[:] = _trim_history(history, MAX_HISTORY_MESSAGES)
        return answer

if __name__ == "__main__":
    # query = "帮我创建一个订单：userId=1, goodId=51, amount=56, addressBookId=1"
    chat_history: List[Dict] = []
    while True:
        query = input("请输入您的需求：")
        if not query:
            break
        if query.strip() in {"/reset", "/clear"}:
            chat_history.clear()
            print("上下文已清空。")
            continue
        print(asyncio.run(run_agent_once(query, chat_history)))