"""Regex demo playground.

通过注释切换 DEMO：
1. 在 DEMOS 字典中取消某个 demo 的注释。
2. 运行 python .\main.py 查看结果。
"""

import re


def run_demo(title: str, pattern: str, samples: list[str]) -> None:
	print(f"\n=== {title} ===")
	print(f"pattern: {pattern}")
	for s in samples:
		matched = bool(re.fullmatch(pattern, s))
		print(f"{s!r:14} -> {matched}")


DEMOS = {
	# 1) raw string + 开头/结尾锚点 + 字符集合 + 一次或多次
	"base_allowed_chars_plus": {
		"pattern": r"^[a-zA-Z0-9_() ]+$",
		"samples": ["hello_123", "A(1)", "bad-1", "", "中文"],
		"note": "只允许字母数字下划线括号空格，且至少1个字符",
	},

	# 2) 去掉 + 后：只能匹配 1 个字符
	"without_plus_single_char_only": {
		"pattern": r"^[a-zA-Z0-9_() ]$",
		"samples": ["A", "_", "_123", "ab", " "],
		"note": "无 + 时，整串只能是一个合法字符",
	},

	# 3) + / * / ? 快速对比
	"quantifier_plus": {
		"pattern": r"^a+$",
		"samples": ["a", "aa", "", "b"],
		"note": "a+: 至少一个 a",
	},
	"quantifier_star": {
		"pattern": r"^a*$",
		"samples": ["a", "aa", "", "b"],
		"note": "a*: 零个或多个 a",
	},
	"quantifier_question": {
		"pattern": r"^a?$",
		"samples": ["a", "", "aa", "b"],
		"note": "a?: 零个或一个 a",
	},

	# 4) [] 是单字符多选一
	"char_class_abc": {
		"pattern": r"^[abc]$",
		"samples": ["a", "b", "c", "ab", "d"],
		"note": "[abc] 匹配一个字符：a/b/c",
	},

	# 5) abc 是固定连续子串
	"fixed_substring_abc": {
		"pattern": r"^abc$",
		"samples": ["abc", "abbc", "zabc", "ab"],
		"note": "abc 必须按顺序连续出现",
	},

	# 6) [abc]bc 对比
	"char_class_then_bc": {
		"pattern": r"^[abc]bc$",
		"samples": ["abc", "bbc", "cbc", "dbc", "abbc"],
		"note": "首字符是 a/b/c，后面固定 bc",
	},

	# 7) a[bc]d 对比
	"a_bc_d": {
		"pattern": r"^a[bc]d$",
		"samples": ["abd", "acd", "abcd", "aad"],
		"note": "中间字符是 b 或 c",
	},
}


if __name__ == "__main__":
	# 默认演示：先跑基础示例
	enabled = [
		"base_allowed_chars_plus",
		# "without_plus_single_char_only",
		# "quantifier_plus",
		# "quantifier_star",
		# "quantifier_question",
		# "char_class_abc",
		# "fixed_substring_abc",
		# "char_class_then_bc",
		# "a_bc_d",
	]

	for name in enabled:
		demo = DEMOS[name]
		print(f"note: {demo['note']}")
		run_demo(name, demo["pattern"], demo["samples"])

