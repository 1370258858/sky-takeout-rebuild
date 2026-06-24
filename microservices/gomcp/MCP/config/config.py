import json
import os
from pathlib import Path

from openai import OpenAI

# ---------------------------------------------------------------------------
# 路由配置：从mcp_tool_routes.json 读取各服务 URL
# 当前写法线程不安全，需要改进
# ---------------------------------------------------------------------------

_routes_path = Path(__file__).parent / "mcp_tool_routes.json"

with open(_routes_path, "r") as _f:
    _json_data = json.load(_f)

_services_cfg: dict = _json_data.get("services", {})


def resolve_url(service_name: str) -> str:
    """根据 service_name 从配置中解析实际 URL（优先读环境变量）"""
    cfg = _services_cfg.get(service_name, {})
    env_name = cfg.get("urlEnv")
    default_url = cfg.get("defaultUrl")
    if env_name:
        return os.getenv(env_name, default_url)
    return default_url



def get_seesion(service_name: str) -> list:
    """根据 service_name 从配置中解析实际 session（优先读环境变量）"""
    cfg = _services_cfg.get(service_name, {})
    session_list = cfg.get("sessionList")
    # 若配置中无 sessionList，返回空列表或默认值
    if session_list is None:
        return []
    return session_list

# 会话落盘
def save_session(service_name: str, session_id: str, history: list) -> None:
    """保存对话历史到 mcp_tool_routes.json 中对应 session 的 data 字段。"""
    cfg = _services_cfg.get(service_name, {})
    session_list = cfg.get("sessionList", [])
    if not isinstance(session_list, list):
        session_list = []
        cfg["sessionList"] = session_list


    # 找到对应 sessionId 的 session，更新其 data 字段
    for session in session_list:
        if str(session.get("sessionId")) == session_id:
            session["data"] = history
            break
    else:
        session_list.append({"sessionId": session_id, "data": history})

    # 将更新后的数据写回 JSON 文件
    with open(_routes_path, "w", encoding="utf-8") as f:
        json.dump(_json_data, f, ensure_ascii=False, indent=2)




# ---------------------------------------------------------------------------
# LLM 配置
# ---------------------------------------------------------------------------
MODEL_NAME: str = os.getenv("MODEL_NAME", "deepseek-v4-pro")
MODEL_MAX_HISTORY: int = int(os.getenv("MODEL_MAX_HISTORY", "10"))

llm = OpenAI(
    api_key=os.getenv("LLM_API_KEY", "sk-3dad08434cf2403199dce62cd7c1b972"),
    base_url=os.getenv("LLM_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
)

# ---------------------------------------------------------------------------
# System Prompt
# ---------------------------------------------------------------------------
SYSTEM_PROMPT = (
    "你是订单助手。若需要下单请调用 create_order 工具。"
    "若需要查看购物车请调用 cart_detail 工具。"
    "若需要修改购物车请调用 update_cart 工具。"
    "若需要删除购物车请调用 delete_cart 工具。"
    "若需要查看商品列表请调用 list_goods 工具。"
    "创建订单时禁止臆造 userId。若用户未提供 userId，"
    "请在 create_order 参数中传 userId=0，由 MCP 服务端通过 Elicit 继续补充。"
    "工具返回后再给最终中文答复。"
)
