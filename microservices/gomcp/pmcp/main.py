import os
import json
import asyncio
from contextlib import AsyncExitStack
from typing import List, Dict, Optional
from openai import OpenAI

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from a import normalize_mcp_result , get_field,mcp_tool_to_openai_schema,add_to_history,_parse_tool_arguments

# ============ 步骤 1: 加载 MCP 服务配置 ============
# 读取json文件
with open("./mcp_tool_routes.json", "r") as f:
    json_data = json.load(f)

services_cfg = json_data.get("services", {})

def _resolve_url(service_name: str) -> str:
    """从配置文件或环境变量中解析 MCP 服务 URL"""
    cfg = services_cfg.get(service_name, {})
    env_name = cfg.get("urlEnv")
    default_url = cfg.get("defaultUrl")
    if env_name:
        return os.getenv(env_name, default_url)
    return default_url

# 解析三个微服务的 MCP 服务器地址
MCP_ORDER_SERVER_URL = _resolve_url("order")
MCP_GOODS_SERVER_URL = _resolve_url("goods")
MCP_DELIVERY_SERVER_URL = _resolve_url("delivery")

# ============ 步骤 2: 初始化 LLM 模型 ============
MODEL = os.getenv("LLM_MODEL","deepseek-v4-pro")
MAX_HISTORY_MESSAGES = int(os.getenv("MAX_HISTORY_MESSAGES", "30"))  # 对话历史最大保留条数

# 使用阿里云 DashScope 兼容 OpenAI API
llm = OpenAI( api_key="sk-3dad08434cf2403199dce62cd7c1b972",
    base_url="https://dashscope.aliyuncs.com/compatible-mode/v1",)

# LLM 系统提示词，指导模型何时调用哪些工具
SYSTEM_PROMPT = (
    "你是订单助手。若需要下单请调用 create_order 工具。"
    "若需要查看购物车请调用 cart_detail 工具。"
    "若需要修改购物车请调用 update_cart 工具。"
    "若需要删除购物车请调用 delete_cart 工具。"
    "若需要查看商品列表请调用 list_goods 工具。"
    "工具返回后再给最终中文答复。"
)






def _tool_result_to_payload(tool_result):
    """将 MCP 工具执行结果规范化，以便回传给 LLM"""
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


def _trim_history(history: List[Dict], max_messages: int) -> List[Dict]:
    """限制对话历史的长度，防止上下文过长"""
    if len(history) <= max_messages:
        return history
    return history[-max_messages:]

# 帮我单一个鸡排订单，要有两块鸡排，一瓶可乐，
# 1.mcp提供商品总列表（包括库存余量,也就是检查鸡排库存大于2，可乐大于1)，session 获得了这个mcp接口， llm填充参数，下单
# 2.返回结果
# - 把mcp 协议商品总列表 暴露出来，定义请求参数(可复用) 
# - llm 判断库存是否够，够的话 ，这个应该通过提示词告诉llm，下单前的检查步骤
# - 调用下单mcp

# ============ 步骤 4: 主代理函数 ============
async def run_agent_once(user_query: str, history: Optional[List[Dict]] = None):
    """执行一次代理循环：用户输入 -> LLM 判断 -> 调用工具 -> 生成回复"""
    if history is None:
        history = []

    # 步骤 4.1: 并发连接多个 MCP 微服务，使用 AsyncExitStack 管理异步上下文生命周期
    async with AsyncExitStack() as stack:
        # 建立 HTTP 双向流连接
        order_read, order_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_ORDER_SERVER_URL))
        goods_read, goods_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_GOODS_SERVER_URL))
        delivery_read, delivery_write, _ = await stack.enter_async_context(streamablehttp_client(MCP_DELIVERY_SERVER_URL))

        # 为每个微服务创建 MCP 客户端会话
        order_session = await stack.enter_async_context(ClientSession(order_read, order_write))
        goods_session = await stack.enter_async_context(ClientSession(goods_read, goods_write))
        delivery_session = await stack.enter_async_context(ClientSession(delivery_read, delivery_write))

        # 初始化所有会话
        await order_session.initialize()
        await goods_session.initialize()
        await delivery_session.initialize()

        # 步骤 4.2: 从所有 MCP 服务汇总可用工具，并记录每个工具对应的源服务
        tool_to_session = {}  # 工具名 -> MCP 会话的映射
        tools = []
        for session in (order_session, goods_session, delivery_session):
            result = await session.list_tools()  # 获取该服务的所有工具
            service_tools = get_field(result, "tools", default=result) or []
            for t in service_tools:
                name = get_field(t, "name")
                if name and name not in tool_to_session:
                    tool_to_session[name] = session  # 记录工具源
                    tools.append(t)

        # 转换成 OpenAI API 兼容的函数定义
        llm_tools = [mcp_tool_to_openai_schema(t) for t in tools]
        print(f"工具清单：{llm_tools} \n")

        # 步骤 4.3: 构建消息历史，包含系统提示、对话历史和新用户输入
       history_messages = [{"role": "system", "content": SYSTEM_PROMPT}, *history, {"role": "user", "content": user_query}]

        # 步骤 4.4: 第一轮 LLM 调用：判断是否需要调用工具
        resp = llm.chat.completions.create( 
            model=MODEL,
            messages=history_messages,
            tools=llm_tools,  # 提供可用工具列表给 LLM
            temperature=0,
        )
        msg = resp.choices[0].message
        print("LLM 回复：{}", msg.content or "", "工具调用：", msg.tool_calls or [])
        tool_calls = msg.tool_calls or []

        # 步骤 4.5: 若 LLM 决定调用工具，则执行工具调用
        if tool_calls:
            # 将 LLM 助手消息和工具调用追加到消息历史
            add_to_history(history_messages,msg,tool_calls)
            # 步骤 4.6: 逐个执行工具调用，路由到对应的 MCP 会话
            for tc in tool_calls:
                name = tc.function.name
                args = _parse_tool_arguments(tc.function.arguments)

                # 根据工具名查找目标 MCP 会话，默认使用 order_session
                target_session = tool_to_session.get(name, order_session)
                try:
                    # 调用远程 MCP 工具
                    tool_result = await target_session.call_tool(name, arguments=args)
                    # 规范化 MCP 返回结果
                    payload = normalize_mcp_result(tool_result)
                except Exception as e:
                    payload = {
                        "ok": False,
                        "error": f"tool {name} failed: {e}",
                        "arguments": args,
                    }

                # 将工具执行结果转为 JSON 字符串，回填给 LLM
                content_text = json.dumps(payload, ensure_ascii=False)

                add_to_history.append({
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": content_text,
                })

            # 步骤 4.7: 第二轮 LLM 调用：让 LLM 基于工具结果生成最终回复
            final_resp = llm.chat.completions.create(
                model=MODEL,
                messages=add_to_history,
                temperature=0,
            )
            final_text = final_resp.choices[0].message.content or "未生成回答"
            # 将本轮对话追加到历史记录
            history.append({"role": "user", "content": user_query})
            history.append({"role": "assistant", "content": final_text})
            history[:] = _trim_history(history, MAX_HISTORY_MESSAGES)
            return final_text

        # 步骤 4.8: 若 LLM 无需调用工具，直接返回 LLM 回复
        answer = msg.content or "未生成回答"
        history.append({"role": "user", "content": user_query})
        history.append({"role": "assistant", "content": answer})
        history[:] = _trim_history(history, MAX_HISTORY_MESSAGES)
        return answer

# ============ 步骤 5: 主程序入口 ============
if __name__ == "__main__":
    # 示例用户输入: "帮我创建一个订单：userId=1, goodId=51, amount=56, addressBookId=1"
    chat_history: List[Dict] = []  # 维护对话历史以支持多轮对话
    while True:
        query = input("请输入您的需求：")
        if not query:
            break
        # 支持清空历史的特殊命令
        if query.strip() in {"/reset", "/clear"}:
            chat_history.clear()
            print("上下文已清空。")
            continue
        # 执行代理循环，并打印返回结果
        print(asyncio.run(run_agent_once(query, chat_history)))