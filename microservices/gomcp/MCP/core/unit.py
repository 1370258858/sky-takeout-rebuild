import json
from typing import Any

def normalize_mcp_result(result: Any) -> Any:
    """Return the model-facing payload from an MCP call result."""
    if isinstance(result, dict):
        if "structuredContent" in result:
            return result["structuredContent"]
        if "structured_content" in result:
            return result["structured_content"]
        return result

    structured = get_field(result, "structuredContent", "structured_content")
    if structured is not None:
        return structured

    content = get_field(result, "content", default=None)
    if content:
        texts = []
        for item in content:
            text = get_field(item, "text", default=None)
            if text is not None:
                texts.append(text)
        if len(texts) == 1:
            try:
                return json.loads(texts[0])
            except json.JSONDecodeError:
                return texts[0]
        if texts:
            return "\n".join(texts)

    return result

def get_field(obj, *names, default=None):
    for n in names:
        if isinstance(obj, dict) and n in obj:
            return obj[n]
        if hasattr(obj, n):
            return getattr(obj, n)
    return default


# ============ 步骤 3: 工具转换函数 ============
def mcp_tool_to_openai_schema(tool):
    """将 MCP 工具定义转换成 OpenAI API 兼容格式"""
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


def add_to_history(history_message,current_message,tool_calls):
                history_message.append({
                "role": "assistant",
                "content": current_message.content or "",
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



def _parse_tool_arguments(raw_args):
    """解析 LLM 传入的工具参数，确保为有效的 JSON 对象"""
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
