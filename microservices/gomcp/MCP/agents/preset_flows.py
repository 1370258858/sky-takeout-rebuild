from typing import Any, Callable, Dict, List, Optional


REPEAT_LAST_ORDER_INTENT = "repeat_last_order"
REPEAT_LAST_ORDER_LOOKUP_STAGE = "lookup_last_order"
REPEAT_LAST_ORDER_CREATE_STAGE = "create_order"
REPEAT_LAST_ORDER_DONE_STAGE = "done"


def is_repeat_last_order_intent(query: str) -> bool:
    return (query or "").strip() == "1"


def build_repeat_last_order_lookup_state(
    user_id: int,
    build_tool_call: Callable[[str, Dict[str, Any]], Any],
) -> Dict[str, Any]:
    preset_calls = [build_tool_call("list_orders", {"userId": user_id})]
    return {
        "intent_source": "preset",
        "preset_intent": REPEAT_LAST_ORDER_INTENT,
        "preset_stage": REPEAT_LAST_ORDER_LOOKUP_STAGE,
        "tool_calls": preset_calls,
        "last_tool_names": [],
        "tool_payloads": [],
        "runtime_tool_messages": [],
        "reply": "",
        "should_retry": False,
        "retry_reason": {},
    }


def extract_last_order(payloads: List[Any]) -> Optional[Dict[str, Any]]:
    for payload in payloads or []:
        if not isinstance(payload, dict):
            continue
        orders = payload.get("orders")
        if isinstance(orders, list) and orders:
            last = orders[-1]
            return last if isinstance(last, dict) else None
    return None


def plan_repeat_last_order_follow_up(
    tool_payloads: List[Any],
    last_tool_names: List[str],
    preset_stage: str,
    resolve_user_id: Callable[[], int],
    build_tool_call: Callable[[str, Dict[str, Any]], Any],
) -> Dict[str, Any]:
    if "list_orders" in last_tool_names and preset_stage == REPEAT_LAST_ORDER_LOOKUP_STAGE:
        last_order = extract_last_order(tool_payloads)
        if not isinstance(last_order, dict):
            return {
                "reply": "没有找到可复用的历史订单，请告诉我商品和地址信息，我来帮你下单。",
                "tool_calls": [],
                "preset_stage": REPEAT_LAST_ORDER_DONE_STAGE,
                "intent_source": "llm",
                "preset_intent": "",
            }

        good_ids = last_order.get("goodIds")
        address_book_id = last_order.get("addressBookId")
        amount = last_order.get("amount")
        if not isinstance(good_ids, list) or not good_ids or not isinstance(address_book_id, int) or address_book_id <= 0:
            return {
                "reply": "我找到了上一单，但缺少可复用的商品或地址信息，请补充后我再下单。",
                "tool_calls": [],
                "preset_stage": REPEAT_LAST_ORDER_DONE_STAGE,
                "intent_source": "llm",
                "preset_intent": "",
            }

        create_args: Dict[str, Any] = {
            "userId": int(last_order.get("userId") or resolve_user_id()),
            "goodIds": good_ids,
            "addressBookId": int(address_book_id),
        }
        if isinstance(amount, (int, float)):
            create_args["amount"] = float(amount)
        address = last_order.get("address")
        if isinstance(address, str) and address.strip():
            create_args["address"] = address.strip()

        return {
            "tool_calls": [build_tool_call("create_order", create_args)],
            "preset_stage": REPEAT_LAST_ORDER_CREATE_STAGE,
        }

    if "create_order" in last_tool_names and preset_stage == REPEAT_LAST_ORDER_CREATE_STAGE:
        created_payload = (tool_payloads or [{}])[-1]
        order_no = ""
        if isinstance(created_payload, dict):
            num = created_payload.get("number")
            if isinstance(num, str):
                order_no = num
        reply = "已按老样子为你再下一单。"
        if order_no:
            reply = f"已按老样子为你再下一单，订单号 {order_no}。"
        return {
            "reply": reply,
            "tool_calls": [],
            "last_tool_names": [],
            "preset_stage": REPEAT_LAST_ORDER_DONE_STAGE,
            "intent_source": "llm",
            "preset_intent": "",
        }

    return {}


def resolve_preset_state(
    query: str,
    resolve_user_id: Callable[[], int],
    build_tool_call: Callable[[str, Dict[str, Any]], Any],
) -> Optional[Dict[str, Any]]:
    if is_repeat_last_order_intent(query):
        return build_repeat_last_order_lookup_state(resolve_user_id(), build_tool_call)
    return None


def finalize_preset_state(
    state: Dict[str, Any],
    resolve_user_id: Callable[[], int],
    build_tool_call: Callable[[str, Dict[str, Any]], Any],
) -> Dict[str, Any]:
    if (state.get("intent_source") or "") != "preset":
        return {}
    if (state.get("preset_intent") or "") != "repeat_last_order":
        return {}
    return plan_repeat_last_order_follow_up(
        tool_payloads=state.get("tool_payloads") or [],
        last_tool_names=state.get("last_tool_names") or [],
        preset_stage=state.get("preset_stage") or "",
        resolve_user_id=resolve_user_id,
        build_tool_call=build_tool_call,
    )