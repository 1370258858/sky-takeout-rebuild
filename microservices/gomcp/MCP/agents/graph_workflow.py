import json
from datetime import datetime, timezone
from types import SimpleNamespace
from uuid import uuid4
from typing import Any, Dict, List, Literal, TypedDict

from langgraph.graph import END, StateGraph
from jsonschema import Draft202012Validator, ValidationError

from core.unit import _parse_tool_arguments, normalize_mcp_result
from rag.retrieve import SlangRetrievalMixin
from .preset_flows import finalize_preset_state, resolve_preset_state


class RetryInfo(TypedDict, total=False):
    retry_times: int
    message: str
    timestamp: str
    details: str


class AgentGraphState(TypedDict, total=False):
    query: str
    turn: int
    intent_source: str
    preset_intent: str
    preset_stage: str
    order_state: str
    event: str
    tool_calls: List[Any]
    last_tool_names: List[str]
    tool_payloads: List[Any]
    runtime_tool_messages: List[Dict[str, Any]]
    need_slang_retrieval: bool
    slang_retrieval_done: bool
    slang_query: str
    slang_tool_calls: List[Any]
    slang_tool_payloads: List[Any]
    slang_retrieval_candidates: List[Dict[str, Any]]
    slang_selected_sku: str
    slang_selected_product: str
    slang_retrieval_confidence: float
    reply: str
    should_exit: bool
    should_retry: bool
    retry_reason: RetryInfo


class GraphWorkflowMixin(SlangRetrievalMixin):

    def _tool_call_name(self, tc: Any) -> str:
        if isinstance(tc, dict):
            fn = tc.get("function")
            if isinstance(fn, dict):
                name = fn.get("name")
                return name if isinstance(name, str) else ""
        fn = getattr(tc, "function", None)
        name = getattr(fn, "name", "")
        return name if isinstance(name, str) else ""

    def _tool_call_id(self, tc: Any) -> str:
        if isinstance(tc, dict):
            tcid = tc.get("id")
            return tcid if isinstance(tcid, str) else f"call_{uuid4().hex[:24]}"
        tcid = getattr(tc, "id", "")
        return tcid if isinstance(tcid, str) and tcid else f"call_{uuid4().hex[:24]}"

    def _tool_call_args(self, tc: Any) -> Dict[str, Any]:
        if isinstance(tc, dict):
            fn = tc.get("function")
            if isinstance(fn, dict):
                return _parse_tool_arguments(fn.get("arguments"))
            return {}
        fn = getattr(tc, "function", None)
        args = getattr(fn, "arguments", "{}")
        return _parse_tool_arguments(args)

    def _build_tool_call(self, name: str, arguments: Dict[str, Any]) -> Any:
        return SimpleNamespace(
            id=f"call_{uuid4().hex[:24]}",
            function=SimpleNamespace(
                name=name,
                arguments=json.dumps(arguments, ensure_ascii=False),
            ),
        )
    # user id 简化  直接iget_user().get("user_id") 否则默认456
    def _resolve_user_id(self) -> int:
        user_snapshot = self.factMemory.get_user() if self.factMemory else {}
        user_id = user_snapshot.get("user_id") if isinstance(user_snapshot, dict) else 0
        return user_id if isinstance(user_id, int) and user_id > 0 else 456

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
            name = self._tool_call_name(tc)
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

    def _route_after_planner(self, state: AgentGraphState) -> Literal["guard", "retrieval", "transition"]:
        if state.get("need_slang_retrieval", False) and not state.get("slang_retrieval_done", False):
            return "retrieval"
        tool_calls = state.get("tool_calls") or []
        return "guard" if tool_calls else "transition"

    def _route_after_guard(self, state: AgentGraphState) -> Literal["tool", "transition"]:
        tool_calls = state.get("tool_calls") or []
        return "tool" if tool_calls else "transition"

    def _route_after_tool(self, state: AgentGraphState) -> Literal["fact_update", "recover"]:
        if state.get("should_retry", False):
            return "recover"
  
        return "fact_update"

    def _route_after_retrieval(self, state: AgentGraphState) -> Literal["planner", "recover"]:
        if state.get("should_retry", False):
            return "recover"
        return "planner"


    def _route_after_recover(self, state: AgentGraphState) -> Literal["planner", "transition"]:
        # 有回复时直接走 transition 收尾；否则继续回 planner 重试。
        if state.get("reply"):
            return "transition"
        return "planner"

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
        factresult = (False, False)
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
        # 代码扩展为 解析送达时间意图/等用户意图

        self.memory.add("user", query)
        # 如果有预设意图，则直接返回预设状态 ，走快流程
        preset_state = resolve_preset_state(query, self._resolve_user_id, self._build_tool_call)
        if preset_state:
            self._log_op(
                op="preset_router_hit",
                input_summary={"query": query},
                output_summary={
                    "intent_source": "preset",
                    "preset_intent": "repeat_last_order",
                    "tool_call_count": len(preset_state.get("tool_calls") or []),
                },
            )
            return preset_state

        self._log_op(
            op="fact_extract",
            input_summary={"query_len": len(query)},
            output_summary={
                "has_budget": bool(factresult[0]),
                "has_delivery_time": bool(factresult[1]),
            },
        )
        return {
            "intent_source": "llm",
            "preset_intent": "",
            "preset_stage": "",
            "need_slang_retrieval": False,
            "slang_retrieval_done": False,
            "slang_query": query,
            "slang_tool_calls": [],
            "slang_tool_payloads": [],
            "slang_retrieval_candidates": [],
            "slang_selected_sku": "",
            "slang_selected_product": "",
            "slang_retrieval_confidence": 0.0,
            "tool_calls": [],
            "last_tool_names": [],
            "tool_payloads": [],
            "runtime_tool_messages": [],
            "reply": "",
            "should_retry": False,
            "retry_reason": {},
        }

    def _node_planner(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="planner")
        intent_source = state.get("intent_source") or "llm"
        if intent_source == "preset":
            preset_tool_calls = state.get("tool_calls") or []
            if preset_tool_calls:
                memory_tool_calls: List[Dict[str, Any]] = []
                for tc in preset_tool_calls:
                    raw_args = self._tool_call_args(tc)
                    memory_tool_calls.append(
                        {
                            "id": self._tool_call_id(tc),
                            "type": "function",
                            "function": {
                                "name": self._tool_call_name(tc),
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
                        "source": "preset",
                        "tool_call_count": len(preset_tool_calls),
                        "preset_stage": state.get("preset_stage", ""),
                    },
                )
                return {
                    "tool_calls": list(preset_tool_calls),
                    "runtime_tool_messages": [],
                    "reply": "",
                    "should_retry": False,
                    "retry_reason": {},
                }

        runtime_tool_messages = state.get("runtime_tool_messages") or []
        resp = self._llm_call(extra_messages=runtime_tool_messages)
        # 这里的重试是给retry机制用的，主要是针对planner返回的结果不符合json schema的情况
        if resp is None:
            retry_reason: RetryInfo = {
                "retry_times": int((state.get("retry_reason") or {}).get("retry_times", 0)),
                "message": "planner response validate failed",
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "details": "llm response did not pass json schema validation",
            }
            return {
                "should_retry": True,
                "retry_reason": retry_reason,
                "tool_calls": list(state.get("tool_calls") or []),
                "runtime_tool_messages": [],
                "reply": "",
            }
        choice = resp.choices[0] if resp.choices else None
        payload = self._extract_payload_from_resp(resp) if hasattr(self, "_extract_payload_from_resp") else {}
        payload = payload if isinstance(payload, dict) else {}
        tool_calls = choice.message.tool_calls if choice and choice.message else None

        need_slang_retrieval = bool(payload.get("need_slang_retrieval", False))
        slang_query = payload.get("slang_query") if isinstance(payload.get("slang_query"), str) else (state.get("query") or "")

        if not tool_calls and isinstance(payload.get("tool_calls"), list):
            structured_tool_calls: List[Any] = []
            for item in payload.get("tool_calls", []):
                if not isinstance(item, dict):
                    continue
                name = item.get("name")
                arguments = item.get("arguments")
                if isinstance(name, str) and isinstance(arguments, dict):
                    structured_tool_calls.append(self._build_tool_call(name, arguments))
            if structured_tool_calls:
                tool_calls = structured_tool_calls

        if tool_calls:
            memory_tool_calls: List[Dict[str, Any]] = []
            for tc in tool_calls:
                raw_args = self._tool_call_args(tc)
                memory_tool_calls.append(
                    {
                        "id": self._tool_call_id(tc),
                        "type": "function",
                        "function": {
                            "name": self._tool_call_name(tc),
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
                "need_slang_retrieval": need_slang_retrieval,
                "slang_query": slang_query,
                "slang_retrieval_done": False,
                "tool_calls": list(tool_calls),
                "runtime_tool_messages": [],
                "reply": "",
                "should_retry": False,
                "retry_reason": {},
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
            "need_slang_retrieval": need_slang_retrieval,
            "slang_query": slang_query,
            "slang_retrieval_done": False,
            "tool_calls": [],
            "runtime_tool_messages": [],
            "reply": reply,
            "should_retry": False,
            "retry_reason": {},
        }

    async def _node_tool(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="tool")
        tool_calls = state.get("tool_calls") or []
        tool_payloads: List[Any] = []
        last_tool_names: List[str] = []
        tool_results_for_merge: List[Dict[str, Any]] = []
        runtime_tool_messages: List[Dict[str, Any]] = []

        for tc in tool_calls:
            tool_name = self._tool_call_name(tc)
            args = self._tool_call_args(tc)
            tc_id = self._tool_call_id(tc)
            if isinstance(tool_name, str) and tool_name:
                last_tool_names.append(tool_name.lower())
            try:
                tool_result = await self.call_tool_by_name(tool_name, args)
                self._log_tool_call(
                    tool_name=tool_name,
                    tool_call_id=tc_id,
                    args=args,
                    result=tool_result,
                )
            except Exception as e:
                self._log_tool_call(
                    tool_name=tool_name,
                    tool_call_id=tc_id,
                    args=args,
                    error=e,

                )
                retry_reason: RetryInfo = {
                    "retry_times": int((state.get("retry_reason") or {}).get("retry_times", 0)),
                    "message": "tool call failed",
                    "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                    "details": f"{tool_name}: {e}",
                }
                return {
                    "should_retry": True,
                    "retry_reason": retry_reason,
                    "tool_calls": list(tool_calls),
                    "runtime_tool_messages": [],
                    "reply": "",
                }

            payload = normalize_mcp_result(tool_result)
            tool_payloads.append(payload)
            runtime_tool_messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tc_id,
                    "content": json.dumps(payload, ensure_ascii=False),
                }
            )
            tool_results_for_merge.append(
                {
                    "id": tc_id,
                    "name": tool_name,
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
            "should_retry": False,
            "retry_reason": {},
        }

    def _node_fact_update(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="fact_update")
        applied = 0
        ignored = 0
        intent_source = state.get("intent_source") or "llm"
        preset_intent = state.get("preset_intent") or ""
        preset_stage = state.get("preset_stage") or ""

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

        preset_result = finalize_preset_state(state, self._resolve_user_id, self._build_tool_call)
        if preset_result:
            reply = preset_result.get("reply")
            if isinstance(reply, str) and reply:
                self.memory.add("assistant", reply)
            return preset_result

        return {}

    def _node_transition(self, state: AgentGraphState) -> AgentGraphState:
        self._set_obs_context(node="transition")
        order_state = state.get("order_state") or self.factMemory.get_fact("order.state", "Draft")
        event_source = state.get("last_tool_names") or state.get("tool_calls") or []
        event = self._derive_event(state.get("query", ""), event_source, order_state)
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
    
    def _retry(self, state: AgentGraphState) -> AgentGraphState:
        """重试节点，用于处理异常情况下的状态重试，和降级澄清"""
        """超过重试次数或者达到最大重试时间后，进行降级处理:兜底回复和用户澄清"""
        """生成失败时，给模型结构化错误，重试一两次"""


        self._set_obs_context(node="recover")
        retry_reason = state.get("retry_reason") or {}
        retry_times = int(retry_reason.get("retry_times", 0))
        max_retry = 2
        # 如果次数达到上限，降级为向用户澄清
        if retry_times >= max_retry:
            fallback = (
                "我尝试执行操作时遇到参数或调用异常。"
                "请确认关键信息（例如 userId、orderId、支付状态）后我再继续。"
            )
            self.memory.add("assistant", fallback)
            self._log_op(
                op="recover_fallback",
                input_summary={"retry_times": retry_times, "reason": retry_reason},
                output_summary={"reply": fallback},
            )
            return {
                "should_retry": False,
                "reply": fallback,
                "tool_calls": [],
                "runtime_tool_messages": [],
            }

        retry_reason["retry_times"] = retry_times + 1
        if "timestamp" not in retry_reason:
            retry_reason["timestamp"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        self._log_op(
            op="recover_retry",
            input_summary={"retry_times": retry_times, "reason": retry_reason},
            output_summary={"next": "planner"},
        )
        return {
            "retry_reason": retry_reason,
            "should_retry": False,
            "reply": "",
            "runtime_tool_messages": [],
        }

    def _node_guard(self, state: AgentGraphState) -> AgentGraphState:
        """守护节点，限制mcp calls，如不可再draft 状态call pay/refund。"""
        self._set_obs_context(node="guard")

        tool_calls = state.get("tool_calls") or []
        if not tool_calls:
            return {"tool_calls": []}

        # 1) 工具白名单：来自 AgentLoop.get_tools() 产生的 OpenAI 工具 schema
        allowed_tool_names = {
            item.get("function", {}).get("name")
            for item in getattr(self, "_llm_tools", [])
            if isinstance(item, dict)
        }

        # 2) 当前状态下禁止调用的工具关键字
        order_state = state.get("order_state") or self.factMemory.get_fact("order.state", "Draft")
        blocked_by_state = {
            "Draft": ["pay", "refund", "delivery"],
            "CollectingInfo": ["pay", "refund", "delivery"],
            "ReadyToCreate": ["pay", "refund", "delivery"],
            "Created": ["refund", "delivery"],
            "Cancelled": ["create", "pay", "refund", "delivery", "update", "delete"],
            "Completed": ["create", "pay", "delivery", "update", "delete"],
        }

        violations: List[str] = []

        # 建立工具入参 schema 索引，用于参数校验
        tool_schema_by_name: Dict[str, Dict[str, Any]] = {}
        for item in getattr(self, "_llm_tools", []):
            if not isinstance(item, dict):
                continue
            fn = item.get("function", {})
            name = fn.get("name")
            params = fn.get("parameters")
            if isinstance(name, str) and isinstance(params, dict):
                tool_schema_by_name[name] = params

        for tc in tool_calls:
            name = self._tool_call_name(tc)
            if not isinstance(name, str) or not name:
                violations.append("tool name missing")
                continue

            lower_name = name.lower()

            # 白名单约束
            if name not in allowed_tool_names:
                violations.append(f"tool not allowed: {name}")
                continue

            # 状态约束
            for keyword in blocked_by_state.get(order_state, []):
                if keyword in lower_name:
                    violations.append(f"tool blocked in state={order_state}: {name}")
                    break

            # 参数 schema 校验
            schema = tool_schema_by_name.get(name)
            if schema is not None:
                args = self._tool_call_args(tc)
                try:
                    Draft202012Validator(schema).validate(args)
                except ValidationError as e:
                    violations.append(f"invalid args for {name}: {e.message}")

        if violations:
            msg = "已拦截不安全/不合法调用: " + " | ".join(violations)
            self._log_op(
                op="guard_block",
                input_summary={"order_state": order_state, "tool_call_count": len(tool_calls)},
                output_summary={"violations": violations},
            )
            self.memory.add("assistant", msg)
            return {
                "tool_calls": [],
                "reply": msg,
                "runtime_tool_messages": [],
                "should_retry": False,
            }

        self._log_op(
            op="guard_pass",
            input_summary={"order_state": order_state, "tool_call_count": len(tool_calls)},
            output_summary={"status": "passed"},
        )
        return {"tool_calls": tool_calls, "should_retry": False}
    

    def _build_graph(self):
        """构建主图：FactExtractor -> Planner -> Retrieval/Guard -> Tool -> FactUpdate/Recover -> Transition。"""
        graph = StateGraph(AgentGraphState)
        graph.add_node("fact_extractor", self._node_fact_extractor)
        graph.add_node("planner", self._node_planner)
        graph.add_node("retrieval", self._node_slang_retrieval)
        graph.add_node("guard", self._node_guard)
        graph.add_node("tool", self._node_tool)
        graph.add_node("fact_update", self._node_fact_update)
        graph.add_node("transition", self._node_transition)
        graph.add_node("recover", self._retry)
        


        graph.set_entry_point("fact_extractor")
        graph.add_edge("fact_extractor", "planner")
        graph.add_conditional_edges(
            "planner",
            self._route_after_planner,
            {
                "retrieval": "retrieval",
                "guard": "guard",
                "transition": "transition",
            },
        )
        graph.add_conditional_edges(
            "retrieval",
            self._route_after_retrieval,
            {
                "planner": "planner",
                "recover": "recover",
            },
        )
        graph.add_conditional_edges(
            "guard",
            self._route_after_guard,
            {
                "tool": "tool",
                "transition": "transition",
            },
        )
        graph.add_conditional_edges(
        "tool", 
        self._route_after_tool,
        {"fact_update": "fact_update",
          "recover": "recover"},
          )
        graph.add_conditional_edges(
                        "recover",
                        self._route_after_recover,
                        {
                                "planner": "planner",
                                "transition": "transition",
                        },
                )
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