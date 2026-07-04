import sys
import unittest
from pathlib import Path
from types import SimpleNamespace

_MCP_ROOT = Path(__file__).resolve().parents[1]
if str(_MCP_ROOT) not in sys.path:
	sys.path.insert(0, str(_MCP_ROOT))

from agents.graph_workflow import GraphWorkflowMixin


class _FakeFactMemory:
	def __init__(self, order_state: str = "Draft"):
		self._order_state = order_state

	def get_fact(self, key: str, default=None):
		if key == "order.state":
			return self._order_state
		return default


class _FakeMemory:
	def __init__(self):
		self.messages = []

	def add(self, role: str, content: str):
		self.messages.append({"role": role, "content": content})


class _GuardHost(GraphWorkflowMixin):
	def __init__(self, llm_tools, order_state="Draft"):
		self._llm_tools = llm_tools
		self.factMemory = _FakeFactMemory(order_state)
		self.memory = _FakeMemory()
		self.ops = []

	def _set_obs_context(self, **kwargs):
		return None

	def _log_op(self, **kwargs):
		self.ops.append(kwargs)


def _tool_call(name: str, arguments: str):
	return SimpleNamespace(
		function=SimpleNamespace(name=name, arguments=arguments),
		id=f"id-{name}",
	)


class TestGuardNode(unittest.TestCase):
	def test_block_by_state_draft_pay(self):
		# 单元测试需要手动构造tools的测试数据
		tools = [
			{
				"type": "function",
				"function": {
					"name": "pay_order",
					"parameters": {"type": "object", "properties": {}, "additionalProperties": True},
				},
			}
		]
		host = _GuardHost(llm_tools=tools, order_state="Draft")
		state = {"tool_calls": [_tool_call("pay_order", "{}")], "order_state": "Draft"}

		out = host._node_guard(state)

		self.assertEqual(out.get("tool_calls"), [])
		self.assertIn("reply", out)
		self.assertIn("blocked in state=Draft", out["reply"])

	def test_block_not_in_whitelist(self):
		tools = [
			{
				"type": "function",
				"function": {
					"name": "create_order",
					"parameters": {"type": "object", "properties": {}, "additionalProperties": True},
				},
			}
		]
		host = _GuardHost(llm_tools=tools, order_state="Created")
		state = {"tool_calls": [_tool_call("unknown_tool", "{}")], "order_state": "Created"}

		out = host._node_guard(state)

		self.assertEqual(out.get("tool_calls"), [])
		self.assertIn("tool not allowed: unknown_tool", out["reply"])

	def test_block_invalid_arguments_schema(self):
		tools = [
			{
				"type": "function",
				"function": {
					"name": "create_order",
					"parameters": {
						"type": "object",
						"properties": {"userId": {"type": "integer"}},
						"required": ["userId"],
						"additionalProperties": False,
					},
				},
			}
		]
		host = _GuardHost(llm_tools=tools, order_state="ReadyToCreate")
		state = {
			"tool_calls": [_tool_call("create_order", '{"userId": "abc"}')],
			"order_state": "ReadyToCreate",
		}

		out = host._node_guard(state)

		self.assertEqual(out.get("tool_calls"), [])
		self.assertIn("invalid args for create_order", out["reply"])

	def test_pass_valid_call(self):
		tools = [
			{
				"type": "function",
				"function": {
					"name": "create_order",
					"parameters": {
						"type": "object",
						"properties": {"userId": {"type": "integer"}},
						"required": ["userId"],
						"additionalProperties": False,
					},
				},
			}
		]
		host = _GuardHost(llm_tools=tools, order_state="ReadyToCreate")
		call = _tool_call("create_order", '{"userId": 1}')
		state = {"tool_calls": [call], "order_state": "ReadyToCreate"}

		out = host._node_guard(state)

		self.assertEqual(out.get("tool_calls"), [call])
		self.assertNotIn("reply", out)


if __name__ == "__main__":
	unittest.main(verbosity=2)
