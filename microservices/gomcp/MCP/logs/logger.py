import json
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional


class Logger:
    """Logging mixin for AgentLoop/graph workflow."""

    def _set_obs_context(
        self,
        *,
        session_id: Optional[str] = None,
        turn: Optional[int] = None,
        trace_id: Optional[str] = None,
        node: Optional[str] = None,
    ) -> None:
        if session_id is not None:
            self._obs_context["session_id"] = str(session_id)
        if turn is not None:
            self._obs_context["turn"] = turn
        if trace_id is not None:
            self._obs_context["trace_id"] = trace_id
        if node is not None:
            self._obs_context["node"] = node

    def _log_op(
        self,
        *,
        op: str,
        status: str = "ok",
        input_summary: Optional[Dict[str, Any]] = None,
        output_summary: Optional[Dict[str, Any]] = None,
        error: Optional[Exception] = None,
    ) -> None:
        entry = {
            "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "kind": "op",
            "op": op,
            "status": status,
            "session_id": self._obs_context.get("session_id"),
            "turn": self._obs_context.get("turn"),
            "trace_id": self._obs_context.get("trace_id"),
            "node": self._obs_context.get("node"),
            "input_summary": input_summary or {},
            "output_summary": output_summary or {},
            "error": str(error) if error is not None else None,
        }
        self._llm_logger.info(json.dumps(entry, ensure_ascii=False))

    def _log_llm_call(
        self,
        messages: List[Dict],
        req: Dict,
        resp: Any = None,
        error: Optional[Exception] = None,
    ) -> None:
        """记录每次 LLM 调用的结构化摘要日志。"""
        finish_reason = None
        tool_call_count = 0
        usage = None
        if resp is not None:
            try:
                choice = resp.choices[0] if resp.choices else None
                finish_reason = getattr(choice, "finish_reason", None)
                msg = getattr(choice, "message", None)
                tc = getattr(msg, "tool_calls", None) if msg else None
                tool_call_count = len(tc) if tc else 0
                usage_obj = getattr(resp, "usage", None)
                if usage_obj is not None and hasattr(usage_obj, "model_dump"):
                    usage = usage_obj.model_dump()
            except Exception:
                usage = None

        entry = {
            "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "kind": "llm",
            "op": "llm_call",
            "status": "error" if error is not None else "ok",
            "session_id": self._obs_context.get("session_id"),
            "turn": self._obs_context.get("turn"),
            "trace_id": self._obs_context.get("trace_id"),
            "node": self._obs_context.get("node"),
            "model": req.get("model"),
            "tool_choice": req.get("tool_choice"),
            "context_message_count": len(messages),
            "finish_reason": finish_reason,
            "tool_call_count": tool_call_count,
            "usage": usage,
            "error": str(error) if error is not None else None,
        }
        self._llm_logger.info(json.dumps(entry, ensure_ascii=False))

    def _log_tool_call(
        self,
        tool_name: str,
        args: Dict[str, Any],
        tool_call_id: Optional[str] = None,
        result: Any = None,
        error: Optional[Exception] = None,
    ) -> None:
        """记录每次 tool 调用的结构化摘要日志。"""
        result_summary: Dict[str, Any] = {}
        if result is not None:
            result_summary = {
                "has_result": True,
                "result_type": type(result).__name__,
            }

        entry = {
            "ts": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "kind": "tool",
            "op": "tool_call",
            "status": "error" if error is not None else "ok",
            "session_id": self._obs_context.get("session_id"),
            "turn": self._obs_context.get("turn"),
            "trace_id": self._obs_context.get("trace_id"),
            "node": self._obs_context.get("node"),
            "tool_name": tool_name,
            "tool_call_id": tool_call_id,
            "args": args,
            "result_summary": result_summary,
            "error": str(error) if error is not None else None,
        }
        self._llm_logger.info(json.dumps(entry, ensure_ascii=False))


