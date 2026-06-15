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