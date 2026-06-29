# Session 管理：会话历史的增加与查找
import json
import re

from datetime import datetime


from pathlib import Path
from typing import Any, Dict, List, Optional
from config.config import get_seesion, save_session


# 会话层配置

# 放最近 N 条对话与工具结果
# 目标：保证当前轮语义连贯
# 生命周期：短，窗口滑动，超出就裁剪
# 典型内容：最近问答、刚刚 tool 返回、澄清中的临时信息

class ConversationMemory:
    """轻量级会话记忆，维护多轮对话历史，并提供增删查能力。"""

    def __init__(self, max_history: int = 10, session_id: Optional[str] = None):
        """
        :param max_history: 保留最近 N 轮（每轮含 user+assistant），
                            超出后丢弃最早的条目（system message 由调用方注入，不在此管理）。
        """
        self._history: List[Dict] = []
        self.max_history = max_history
        self._session_id = session_id

        session_list = get_seesion("order")
        if session_list and session_id is not None:
            # 通过 session_id 从 session_list 中恢复历史数据
            session = next(
                (s for s in session_list if str(s.get("sessionId")) == str(session_id)),
                None,
            )
            if session:
                data = session.get("data", [])
                if isinstance(data, list):
                    self._history = data

        # 启动恢复后立即裁剪，避免首轮 prompt 超出窗口
        if len(self._history) > self.max_history:
            self._history = self._history[-self.max_history :]

    # ------------------------------------------------------------------
    # 增
    # ------------------------------------------------------------------
    def add(self, role: str, content: str, **extra) -> None:
        """追加一条普通消息（user / assistant）。"""
        entry: Dict = {"role": role, "content": content}
        entry.update(extra)
        self._append(entry)

    def add_raw(self, entry: Dict) -> None:
        """直接追加已构造好的消息字典（如 tool 角色消息）。"""
        self._append(entry)

    def _append(self, entry: Dict) -> None:
        self._history.append(entry)
        # 超出容量时，从头部裁剪（保留最近 max_history 条）
        cap = self.max_history
        if len(self._history) > cap:
            self._history = self._history[-cap:]


    def get_history(self) -> List[Dict]:
        """返回完整历史（只读视图）。"""
        return list(self._history)

    def merge_tool_results(self, tool_results: List[Dict[str, Any]]) -> None:
        """可读化地把工具返回按 tool_call_id 回填到最近一条 assistant.tool_calls 中。"""
        if not tool_results:
            return

        by_id: Dict[str, Dict[str, Any]] = {}
        for item in tool_results:
            tcid = item.get("id")
            if isinstance(tcid, str) and tcid:
                by_id[tcid] = item

        if not by_id:
            return

        # 优先回填最近一条 assistant tool_calls
        for msg in reversed(self._history):
            if msg.get("role") != "assistant":
                continue
            calls = msg.get("tool_calls")
            if not isinstance(calls, list):
                continue

            updated = False
            for tc in calls:
                if not isinstance(tc, dict):
                    continue
                tcid = tc.get("id")
                if not isinstance(tcid, str):
                    continue
                merged = by_id.get(tcid)
                if not merged:
                    continue

                fn = tc.get("function")
                if not isinstance(fn, dict):
                    fn = {}
                    tc["function"] = fn
                fn["result"] = merged.get("result")
                updated = True

            if updated:
                return

    def save_history(self, service_name: str = "order") -> None:
        """保存当前历史到持久化存储。"""
        if self._session_id is None:
            return
        save_session(service_name, str(self._session_id), self.get_history())

    def __len__(self) -> int:
        return len(self._history)


class FactMemory:
    """事实记忆：按用户以字典形式保存到 fact.json。"""

    def __init__(self, user_id: Optional[int] = None, fact_file: Optional[Path] = None):
        self._fact_file = fact_file or (Path(__file__).parent.parent / "config" / "fact.json")
        self._doc: Dict[str, Any] = {"users": []}
        self._current: Dict[str, Any] = {"user_id": 0, "facts": {}}
        self._load()

        if user_id is None:
            users = self._doc.get("users", [])
            if users:
                self._current = users[0]
            return

        found = self._find_user(user_id)
        if found is not None:
            self._current = found
        else:
            self._current = {"user_id": int(user_id), "facts": {}}
            self._doc.setdefault("users", []).append(self._current)
            # 新用户初始化时立即落盘，避免测试或后续流程读取不到 user_id。
            self._save()

    def _load(self) -> None:
        if not self._fact_file.exists():
            self._fact_file.parent.mkdir(parents=True, exist_ok=True)
            self._save()
            return

        try:
            with open(self._fact_file, "r", encoding="utf-8") as f:
                payload = json.load(f)
            if isinstance(payload, dict) and isinstance(payload.get("users"), list):
                self._doc = payload
            else:
                self._doc = {"users": []}
        except (json.JSONDecodeError, OSError):
            self._doc = {"users": []}

    def _save(self) -> None:
        self._fact_file.parent.mkdir(parents=True, exist_ok=True)
        with open(self._fact_file, "w", encoding="utf-8") as f:
            json.dump(self._doc, f, ensure_ascii=False, indent=2)

    def _find_user(self, user_id: int) -> Optional[Dict[str, Any]]:
        target = int(user_id)
        for user in self._doc.get("users", []):
            if user.get("user_id") == target:
                return user
        return None

    def get_user(self) -> Dict[str, Any]:
        """获取当前用户的完整记录。"""
        return {
            "user_id": self._current.get("user_id", 0),
            "facts": dict(self._current.get("facts", {})),
        }

    def get_fact(self, key: str, default: Any = None) -> Any:
        """获取单个 fact（返回 value）。"""
        item = self._current.get("facts", {}).get(key)
        if not isinstance(item, dict):
            return default
        return item.get("value", default)
    


    def has_budget_intent(self,  user_input: str = "") -> bool:
        """使用正则判断用户输入是否命中某类意图。"""
        if not user_input:
            return False

        budget_intent_pattern = r"""
        (?:
            预算|价位|价格|多少钱|花费|成本|费用|
            不超(?:过)?|最多|少于|低于|高于|至少|
            不要太贵|别太贵|太贵|贵一点|便宜点|不要太便宜|别太便宜|平价|实惠|经济实惠|
            差不多就行|合适价位|性价比
        )
        |
        (?:\d+(?:\.\d+)?\s*(?:元|块|美元))
        """
        return re.search(budget_intent_pattern, user_input, re.VERBOSE) is not None


    def set_facts_from_user_input(self, user_input: str) -> tuple[bool, bool]:
        "解析用户输入，提取事实信息 返回是否提取到预算和送达时间"
        has_budget = False
        has_delivery_time = False

        # 提取预算信息
        pattern_budget = r"""
        (?:
            (?P<range>\d+(?:\.\d+)?\s*(?:到|-|~)\s*\d+(?:\.\d+)?)
          |
            (?P<upper>(?:不超过|最多|少于|低于)\s*\d+(?:\.\d+)?\s*(?:元|块|美元)?)
          |
            (?P<single>\d+(?:\.\d+)?\s*(?:元|块|美元)?)
        )
        """
        matches = re.finditer(pattern_budget, user_input, re.VERBOSE)
        for match in matches:
            if match.group("range"):
                # 处理范围，结构化保存 min/max 与范围标记
                nums = re.findall(r"\d+(?:\.\d+)?", match.group("range"))
                if len(nums) >= 2:
                    min_v = float(nums[0])
                    max_v = float(nums[1])
                    if min_v > max_v:
                        min_v, max_v = max_v, min_v
                    self.set_fact("budget.min", min_v)
                    self.set_fact("budget.max", max_v)
                    self.set_fact("budget.has_range", True)
                has_budget = True
                pass
            elif match.group("upper") or match.group("single"):
                # 处理上限/单值
                budget_value = match.group("upper") or match.group("single")
                nums = re.findall(r"\d+(?:\.\d+)?", budget_value)
                if nums:
                    self.set_fact("budget.max", float(nums[0]))
                    self.set_fact("budget.has_range", False,)
                has_budget = True
                pass

        # 提取送达时间 
        pattern_delivery = r"(?:在|于)\s*(\d{1,2}:\d{2})\s*(?:之前|以前)"
        matches = re.finditer(pattern_delivery, user_input, re.VERBOSE)
        for match in matches:
            # 处理送达时间
            self.set_fact("delivery.time", match.group(1))
            has_delivery_time = True
            pass

        return has_budget, has_delivery_time


    

    def set_fact(self, key: str, value: Any, confidence: str = "HIGH", source: str = "user_input", updated_at: Optional[str] = None) -> None:
        """设置单个 fact，并写回文件。"""
        facts = self._current.setdefault("facts", {})
        facts[key] = {
            "value": value,
            "confidence": confidence,
            "source": source,
            "updated_at": updated_at or datetime.now().isoformat(),
        }
        self._save()

    def set_facts(self, updates: Dict[str, Dict[str, Any]]) -> None:
        """批量设置 facts，并写回文件。"""
        facts = self._current.setdefault("facts", {})
        for key, meta in updates.items():
            if not isinstance(meta, dict):
                continue
            facts[key] = {
                "value": meta.get("value"),
                "confidence": meta.get("confidence", "HIGH"),
                "source": meta.get("source", "user_input"),
                "updated_at": meta.get("updated_at", ""),
            }
        self._save()
