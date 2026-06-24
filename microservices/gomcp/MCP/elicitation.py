


from mcp.types import ElicitResult

# 导入agent loop 的回调模块
async def elicitation_handler(context, params) -> ElicitResult:
    """处理 MCP 服务端发来的 Elicit 补充信息请求，在终端交互式采集用户输入"""
    print(f"\n[需要补充信息]: {params.message}")
    user_input = {}
    schema = getattr(params, "requestedSchema", None) or {}
    properties = schema.get("properties", {}) if isinstance(schema, dict) else {}

    if properties:
        for field_name, field_info in properties.items():
            desc = field_info.get("description", field_name) if isinstance(field_info, dict) else field_name
            field_type = field_info.get("type", "string") if isinstance(field_info, dict) else "string"
            raw = input(f"  请输入 {desc} ({field_name}): ").strip()
            if not raw:
                continue
            if field_type in ("integer", "number"):
                try:
                    user_input[field_name] = int(raw) if field_type == "integer" else float(raw)
                except ValueError:
                    user_input[field_name] = raw
            elif field_type == "boolean":
                user_input[field_name] = raw.lower() in ("true", "1", "yes", "y")
            else:
                user_input[field_name] = raw
    else:
        # 没有 schema，直接根据 message 提示采集
        raw = input("  请输入值: ").strip()
        if raw:
            # 尝试识别 userId 等关键字段
            key = "userId"
            try:
                user_input[key] = int(raw)
            except ValueError:
                user_input[key] = raw

    if not user_input:
        return ElicitResult(action="cancel", content=None)

    return ElicitResult(action="accept", content=user_input)
