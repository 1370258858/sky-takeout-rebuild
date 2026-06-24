# Session 管理：会话历史的增加与查找
from typing import Dict, List, Optional


class ConversationMemory:
    """轻量级会话记忆，维护多轮对话历史，并提供增删查能力。"""

    def __init__(self, max_history: int = 10,session_id: str = None):
        """
        :param max_history: 保留最近 N 轮（每轮含 user+assistant），
                            超出后丢弃最早的条目（system message 由调用方注入，不在此管理）。
        """
        self._history: List[Dict] = []
        self.max_history = max_history
        self._session_id = session_id

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
        # 超出容量时，从头部裁剪（保留最近 max_history*2 条）
        cap = self.max_history * 2
        if len(self._history) > cap:
            self._history = self._history[-cap:]

    # ------------------------------------------------------------------
    # 查找
    # ------------------------------------------------------------------
    # def find_last(self, role: str) -> Optional[Dict]:
    #     """返回最近一条指定 role 的消息，不存在则返回 None。"""
    #     for entry in reversed(self._history):
    #         if entry.get("role") == role:
    #             return entry
    #     return None

    # def find_all(self, role: str) -> List[Dict]:
    #     """返回所有指定 role 的消息列表。"""
    #     return [e for e in self._history if e.get("role") == role]

    # def find_by_tool_call_id(self, tool_call_id: str) -> Optional[Dict]:
    #     """查找 tool 消息中与 tool_call_id 匹配的条目。"""
    #     for entry in self._history:
    #         if entry.get("role") == "tool" and entry.get("tool_call_id") == tool_call_id:
    #             return entry
    #     return None

    # ------------------------------------------------------------------
    # 取全量 / 清空
    # ------------------------------------------------------------------
    def get_history(self) -> List[Dict]:
        """返回完整历史（只读视图）。"""
        return list(self._history)



    def __len__(self) -> int:
        return len(self._history)
        