import unittest
import sys
from pathlib import Path

_MCP_ROOT = Path(__file__).resolve().parents[1]
if str(_MCP_ROOT) not in sys.path:
    sys.path.insert(0, str(_MCP_ROOT))

from core.memory import FactMemory


class TestFactMemoryIntentAndExtraction(unittest.TestCase):
    def setUp(self) -> None:
        # 直接使用项目内 fact.json，便于运行后直接观察落盘结果。
        self.fact_file = _MCP_ROOT / "config" / "fact.json"
        self.memory = FactMemory(user_id=999, fact_file=self.fact_file)

    def tearDown(self) -> None:
        # 保留写入结果，便于人工检查。
        pass

    # def test_has_budget_intent_true_for_fuzzy_and_numeric(self) -> None:
    #     samples = [
    #         "不要太贵，平价就行",
    #         "预算大概 80 元",
    #         "性价比高一点",
    #         "价格在 100 元左右",
    #         "今天天气不错"
    #     ]
    #     for text in samples:
    #         with self.subTest(text=text):
    #             self.assertTrue(self.memory.has_budget_intent(text))

    # def test_has_budget_intent_false_for_small_talk(self) -> None:
    #     self.assertFalse(self.memory.has_budget_intent("你好，今天天气不错"))


    # def test_set_facts_from_user_input_single_value(self) -> None:
    #     text = "预算 42 元"
    #     has_budget, has_delivery = self.memory.set_facts_from_user_input(text)

    #     self.assertTrue(has_budget)
    #     self.assertFalse(has_delivery)
    #     self.assertEqual(self.memory.get_fact("budget.max"), "42 元")

    def test_set_facts_from_user_input_range(self) -> None:
        text = "12:30 前送达"
        has_budget, has_delivery = self.memory.set_facts_from_user_input(text)

    #     self.assertTrue(has_budget)
    #     self.assertFalse(has_delivery)
    #     self.assertEqual(self.memory.get_fact("budget.min"), 80.0)
    #     self.assertEqual(self.memory.get_fact("budget.max"), 120.0)
    #     self.assertTrue(self.memory.get_fact("budget.has_range"))


if __name__ == "__main__":
    unittest.main(verbosity=2)
