import re


# # 常用案例 2：从一句话里提取“金额”
# # 适用场景：预算识别、订单价格识别、聊天里抽取数字信息
# # 切换演示方式：修改 text 变量后重复运行即可

# text = "我预算大概 80 元，最多不超过 120 元，今天先下单 99.5 元。"

# # 匹配整数或小数，后面跟可选空格和金额单位
# pattern_amount = r"(\d+(?:\.\d+)?)\s*(元|块|rmb|RMB|¥)?"

# matches = re.findall(pattern_amount, text)

# print("匹配结果:", matches)



	

s = "80 元"

part = r"(\d+)(?:\.)\s?(\d+)(元|美元)?"

print(re.match(part, s))






