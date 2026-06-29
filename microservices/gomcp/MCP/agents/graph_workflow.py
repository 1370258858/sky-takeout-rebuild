import json
from datetime import datetime, timezone
from typing import Any, Dict, List, Literal, TypedDict

from langgraph.graph import END, StateGraph

from core.unit import _parse_tool_arguments, normalize_mcp_result


class AgentGraphState(TypedDict, total=False):
    query: str
    turn: int
    order_state: str
    event: str
    tool_calls: List[Any]
    last_tool_names: List[str]
    tool_payloads: List[Any]
    runtime_tool_messages: List[Dict[str, Any]]
    reply: str
    should_exit: bool


class GraphWorkflowMixin:
    def _derive_event(self, query: str, tool_calls: List[Any], order_state: str) -> str:
        """根据本轮输入与工具调用推断状态事件。"""
        q = (query or "").lower()
        if any(word in q for word in ["取消", "cancel", "不要了"]):
            return "USER_CANCEL"

        tool_names = []
        for tc in tool_calls or []:
            if isinstance(tc, str):
                tool_names.append(tc.lower())
                continue
            name = getattr(getattr(tc, "function", None), "name", "")
            if isinstance(name, str):
                tool_names.append(name.lower())

        if any("create_order" in name for name in tool_names):
            return "CREATE_ORDER_SUCCESS"
        if any("pay" in name for name in tool_names):
            return "PAY_SUCCESS"
        if any("delivery" in name for name in tool_names):
            if order_state == "Paid":
                return "DELIVERY_START"
            return "DELIVERY_DONE"

        if order_state in ["Draft", "CollectingInfo"]:
            return "INFO_COLLECTED"
        return "NO_OP"

    def _transition_state(self, state: str, event: str) -> str:
        """显式状态机转移表。"""
        table = {
            "Draft": {
                "INFO_COLLECTED": "CollectingInfo",
                "USER_CANCEL": "Cancelled",
            },
            "CollectingInfo": {
                "INFO_COLLECTED": "ReadyToCreate",
                "CREATE_ORDER_SUCCESS": "Created",
                "USER_CANCEL": "Cancelled",
            },
            "ReadyToCreate": {
                "CREATE_ORDER_SUCCESS": "Created",
                "USER_CANCEL": "Cancelled",
            },
            "Created": {
                "PAY_SUCCESS": "Paid",
                "USER_CANCEL": "Cancelled",
            },
            "Paid": {
                "DELIVERY_START": "Delivering",
            },
            "Delivering": {
                "DELIVERY_DONE": "Completed",
            },
            "Completed": {},
            "Cancelled": {},
        }
        next_state = table.get(state, {}).get(event)
        return next_state if next_state is not None else state

    def _route_after_planner(self, state: AgentGraphState) -> Literal["tool", "transition"]:
        tool_calls = state.get("tool_calls") or []
        return "tool" if tool_calls else "transition"

    def _route_after_transition(self, state: AgentGraphState) -> Literal["planner", "end"]:
        if state.get("should_exit", False):
            return "end"
        if state.get("reply"):
            return "end"
        if state.get("last_tool_names"):
            return "planner"
        return "end"

    def _node_fact_extractor(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="fact_extractor")
        query = state.get("query", "")
        # 判断是否有预算意图
        if self.factMemory.has_budget_intent(query):
            factresult = self.factMemory.set_facts_from_user_input(query)
            # 正则提取失败
            if not factresult[0] :
                print("正则表达式检测失败，调用LLM提取预算信息...")
                intentresp = self._llm_get_intent_call(query)
                intent_content = ""
                if intentresp.choices and intentresp.choices[0].message:
                    intent_content = intentresp.choices[0].message.content or ""

                if self._apply_budget_intent_result(intent_content):
                    print("LLM预算意图提取成功，已写入预算事实。")
                else:
                    print("LLM预算意图提取未得到可用结构化结果。")

        self.memory.add("user", query)
        self._log_op(
            op="fact_extract",
            input_summary={"query_len": len(query)},
            output_summary={
                "has_budget": bool(factresult[0]),
                "has_delivery_time": bool(factresult[1]),
            },
        )
        return {
            "tool_calls": [],
            "last_tool_names": [],
            "tool_payloads": [],
            "runtime_tool_messages": [],
            "reply": "",
        }

    def _node_planner(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="planner")
        runtime_tool_messages = state.get("runtime_tool_messages") or []
        resp = self._llm_call(extra_messages=runtime_tool_messages)
        choice = resp.choices[0] if resp.choices else None
        tool_calls = choice.message.tool_calls if choice and choice.message else None

        if tool_calls:
            memory_tool_calls: List[Dict[str, Any]] = []
            for tc in tool_calls:
                raw_args = _parse_tool_arguments(tc.function.arguments)
                memory_tool_calls.append(
                    {
                        "id": tc.id,
                        "type": "function",
                        "function": {
                            "name": tc.function.name,
                            "arguments": json.dumps(raw_args, ensure_ascii=False),
                        },
                    }
                )

            self.memory.add_raw(
                {
                    "role": "assistant",
                    "content": "",
                    "tool_calls": memory_tool_calls,
                }
            )
            self._log_op(
                op="planner_decision",
                output_summary={
                    "route": "tool",
                    "tool_call_count": len(tool_calls),
                },
            )
            return {
                "tool_calls": list(tool_calls),
                "runtime_tool_messages": [],
                "reply": "",
            }

        reply = ""
        if choice and choice.message:
            reply = choice.message.content or ""
            self.memory.add("assistant", reply)

        self._log_op(
            op="planner_decision",
            output_summary={
                "route": "transition",
                "reply_len": len(reply),
            },
        )

        return {
            "tool_calls": [],
            "runtime_tool_messages": [],
            "reply": reply,
        }

    async def _node_tool(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="tool")
        tool_calls = state.get("tool_calls") or []
        tool_payloads: List[Any] = []
        last_tool_names: List[str] = []
        tool_results_for_merge: List[Dict[str, Any]] = []
        runtime_tool_messages: List[Dict[str, Any]] = []

        for tc in tool_calls:
            args = _parse_tool_arguments(tc.function.arguments)
            if isinstance(tc.function.name, str):
                last_tool_names.append(tc.function.name.lower())
            try:
                tool_result = await self.call_tool_by_name(tc.function.name, args)
                self._log_tool_call(
                    tool_name=tc.function.name,
                    tool_call_id=tc.id,
                    args=args,
                    result=tool_result,
                )
            except Exception as e:
                self._log_tool_call(
                    tool_name=tc.function.name,
                    tool_call_id=tc.id,
                    args=args,
                    error=e,
                )
                raise

            payload = normalize_mcp_result(tool_result)
            tool_payloads.append(payload)
            runtime_tool_messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tc.id,
                    "content": json.dumps(payload, ensure_ascii=False),
                }
            )
            tool_results_for_merge.append(
                {
                    "id": tc.id,
                    "name": tc.function.name,
                    "arguments": args,
                    "result": payload,
                }
            )

        self.memory.merge_tool_results(tool_results_for_merge)

        self._log_op(
            op="tool_batch",
            input_summary={"tool_call_count": len(tool_calls)},
            output_summary={"executed_tools": last_tool_names},
        )

        return {
            "tool_payloads": tool_payloads,
            "runtime_tool_messages": runtime_tool_messages,
            "last_tool_names": last_tool_names,
            "tool_calls": [],
        }

    def _node_fact_update(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="fact_update")
        applied = 0
        ignored = 0
        for payload in state.get("tool_payloads") or []:
            if isinstance(payload, dict) and "updateFacts" in payload:
                updates = payload["updateFacts"]
                if isinstance(updates, dict):
                    self.factMemory.set_facts(updates)
                    applied += len(updates)
                else:
                    ignored += 1
                    self._llm_logger.info(
                        json.dumps(
                            {
                                "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                                "kind": "fact_update",
                                "status": "ignored_invalid_shape",
                                "updateFacts_shape": type(updates).__name__,
                            },
                            ensure_ascii=False,
                        )
                    )
        self._log_op(
            op="fact_update",
            output_summary={
                "applied_count": applied,
                "ignored_payload_count": ignored,
            },
        )
        return {}

    def _node_transition(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="transition")
        order_state = state.get("order_state") or self.factMemory.get_fact("order.state", "Draft")
        event_source = state.get("last_tool_names") or state.get("tool_calls") or []
        event = self._derive_event(state.get("query", ""), event_source, )
        next_state = self._transition_state(order_state, event)

        if next_state != order_state:
            self.factMemory.set_fact(
                "order.state",
                next_state,
                confidence="HIGH",
                source="state_machine",
            )

        is_terminal = next_state in ["Completed", "Cancelled"]
        self._log_op(
            op="state_transition",
            input_summary={"from": order_state, "event": event},
            output_summary={"to": next_state, "is_terminal": is_terminal},
        )
        return {
            "order_state": next_state,
            "event": event,
            "should_exit": is_terminal,
        }

    def _build_graph(self):
        """构建五节点 LangGraph：FactExtractor -> Planner -> Tool -> FactUpdate -> Transition。"""
        graph = StateGraph(AgentGraphState)
        graph.add_node("fact_extractor", self._node_fact_extractor)
        graph.add_node("planner", self._node_planner)
        graph.add_node("tool", self._node_tool)
        graph.add_node("fact_update", self._node_fact_update)
        graph.add_node("transition", self._node_transition)

        graph.set_entry_point("fact_extractor")
        graph.add_edge("fact_extractor", "planner")
        graph.add_conditional_edges(
            "planner",
            self._route_after_planner,
            {
                "tool": "tool",
                "transition": "transition",
            },
        )
        graph.add_edge("tool", "fact_update")
        graph.add_edge("fact_update", "transition")
        graph.add_conditional_edges(
            "transition",
            self._route_after_transition,
            {
                "planner": "planner",
                "end": END,
            },
        )
        return graph.compile()