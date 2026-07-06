import json
from datetime import datetime, timezone
from typing import Any, Dict, List, Literal

from langgraph.graph import END, StateGraph

from core.unit import normalize_mcp_result


class SlangRetrievalMixin:
	def _route_after_slang_retrieval_plan(self, state: Dict[str, Any]) -> Literal["tool", "finalize"]:
		tool_calls = state.get("slang_tool_calls") or []
		return "tool" if tool_calls else "finalize"

	def _find_slang_retrieval_tool_name(self) -> str:
		names: List[str] = []
		for item in getattr(self, "_llm_tools", []):
			if not isinstance(item, dict):
				continue
			fn = item.get("function", {})
			name = fn.get("name")
			if isinstance(name, str) and name:
				names.append(name)

		def pick(pred):
			for n in names:
				if pred(n.lower()):
					return n
			return ""

		direct = pick(lambda n: "slang" in n and ("sku" in n or "query" in n or "retrieve" in n or "search" in n))
		if direct:
			return direct

		rag = pick(lambda n: "rag" in n and ("query" in n or "retrieve" in n or "search" in n))
		if rag:
			return rag

		return pick(lambda n: "query" in n or "search" in n)

	def _build_retrieval_args(self, tool_name: str, query: str) -> Dict[str, Any]:
		schema: Dict[str, Any] = {}
		for item in getattr(self, "_llm_tools", []):
			if not isinstance(item, dict):
				continue
			fn = item.get("function", {})
			if fn.get("name") == tool_name and isinstance(fn.get("parameters"), dict):
				schema = fn.get("parameters", {})
				break

		props = schema.get("properties") if isinstance(schema.get("properties"), dict) else {}
		args: Dict[str, Any] = {}

		for key in ["text", "query", "question", "input", "utterance"]:
			if key in props:
				args[key] = query
				break
		if not args:
			args["query"] = query

		for k_key, k_val in [("k", 5), ("top_k", 5), ("n_results", 5), ("limit", 5)]:
			if k_key in props:
				args[k_key] = k_val
				break

		return args

	def _node_slang_retrieval_plan(self, state: Dict[str, Any]) -> Dict[str, Any]:
		self._set_obs_context(node="slang_retrieval_plan")
		if not state.get("need_slang_retrieval", False):
			return {"slang_tool_calls": []}

		query = (state.get("slang_query") or state.get("query") or "").strip()
		if not query:
			return {"slang_tool_calls": []}

		tool_name = self._find_slang_retrieval_tool_name()
		if not tool_name:
			self._log_op(
				op="slang_retrieval_plan",
				output_summary={"route": "skip", "reason": "tool_not_found"},
			)
			return {
				"slang_tool_calls": [],
				"need_slang_retrieval": False,
				"slang_retrieval_done": True,
			}

		args = self._build_retrieval_args(tool_name, query)
		tool_call = self._build_tool_call(tool_name, args)
		self._log_op(
			op="slang_retrieval_plan",
			output_summary={"route": "tool", "tool": tool_name},
		)
		return {"slang_tool_calls": [tool_call]}

	async def _node_slang_retrieval_tool(self, state: Dict[str, Any]) -> Dict[str, Any]:
		self._set_obs_context(node="slang_retrieval_tool")
		tool_calls = state.get("slang_tool_calls") or []
		if not tool_calls:
			return {"slang_tool_payloads": []}

		payloads: List[Any] = []
		runtime_tool_messages: List[Dict[str, Any]] = []
		for tc in tool_calls:
			tool_name = self._tool_call_name(tc)
			args = self._tool_call_args(tc)
			tc_id = self._tool_call_id(tc)
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
				retry_reason = {
					"retry_times": int((state.get("retry_reason") or {}).get("retry_times", 0)),
					"message": "slang retrieval tool call failed",
					"timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
					"details": f"{tool_name}: {e}",
				}
				return {
					"should_retry": True,
					"retry_reason": retry_reason,
					"slang_tool_payloads": [],
				}

			payload = normalize_mcp_result(tool_result)
			payloads.append(payload)
			runtime_tool_messages.append(
				{
					"role": "tool",
					"tool_call_id": tc_id,
					"content": json.dumps(payload, ensure_ascii=False),
				}
			)

		return {
			"slang_tool_payloads": payloads,
			"runtime_tool_messages": runtime_tool_messages,
			"should_retry": False,
		}

	def _node_slang_retrieval_finalize(self, state: Dict[str, Any]) -> Dict[str, Any]:
		self._set_obs_context(node="slang_retrieval_finalize")
		payloads = state.get("slang_tool_payloads") or []
		candidates: List[Dict[str, Any]] = []
		selected_sku = ""
		selected_product = ""
		confidence = 0.0

		def append_candidate(item: Any) -> None:
			if not isinstance(item, dict):
				return
			sku = item.get("sku") or item.get("primary_sku") or item.get("matched_sku")
			product = item.get("product_name") or item.get("name") or item.get("matched_name")
			score = item.get("score") or item.get("confidence") or 0.0
			if isinstance(sku, str) and sku:
				candidates.append(
					{
						"sku": sku,
						"product_name": product if isinstance(product, str) else "",
						"score": float(score) if isinstance(score, (int, float)) else 0.0,
					}
				)

		for payload in payloads:
			if isinstance(payload, dict):
				append_candidate(payload)
				for key in ["candidates", "items", "results", "matches", "data"]:
					value = payload.get(key)
					if isinstance(value, list):
						for it in value:
							append_candidate(it)
			elif isinstance(payload, list):
				for it in payload:
					append_candidate(it)

		if candidates:
			candidates.sort(key=lambda x: x.get("score", 0.0), reverse=True)
			best = candidates[0]
			selected_sku = best.get("sku", "")
			selected_product = best.get("product_name", "")
			confidence = float(best.get("score", 0.0) or 0.0)

		return {
			"need_slang_retrieval": False,
			"slang_retrieval_done": True,
			"slang_retrieval_candidates": candidates,
			"slang_selected_sku": selected_sku,
			"slang_selected_product": selected_product,
			"slang_retrieval_confidence": confidence,
			"slang_tool_calls": [],
			"slang_tool_payloads": [],
		}

	def _build_slang_retrieval_subgraph(self):
		graph = StateGraph(dict)
		graph.add_node("plan", self._node_slang_retrieval_plan)
		graph.add_node("tool", self._node_slang_retrieval_tool)
		graph.add_node("finalize", self._node_slang_retrieval_finalize)

		graph.set_entry_point("plan")
		graph.add_conditional_edges(
			"plan",
			self._route_after_slang_retrieval_plan,
			{
				"tool": "tool",
				"finalize": "finalize",
			},
		)
		graph.add_edge("tool", "finalize")
		graph.add_edge("finalize", END)
		return graph.compile()

	async def _node_slang_retrieval(self, state: Dict[str, Any]) -> Dict[str, Any]:
		subgraph = self._build_slang_retrieval_subgraph()
		result = await subgraph.ainvoke(state)
		return {
			"need_slang_retrieval": result.get("need_slang_retrieval", False),
			"slang_retrieval_done": result.get("slang_retrieval_done", True),
			"slang_retrieval_candidates": result.get("slang_retrieval_candidates", []),
			"slang_selected_sku": result.get("slang_selected_sku", ""),
			"slang_selected_product": result.get("slang_selected_product", ""),
			"slang_retrieval_confidence": result.get("slang_retrieval_confidence", 0.0),
			"runtime_tool_messages": result.get("runtime_tool_messages", []),
			"should_retry": result.get("should_retry", False),
			"retry_reason": result.get("retry_reason", {}),
		}
