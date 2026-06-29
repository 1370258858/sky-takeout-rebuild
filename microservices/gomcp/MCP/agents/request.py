from typing import Any, Optional, Protocol

from config.config import GET_INTENT_MODEL_NAME, MODEL_NAME


class RequestBuilder(Protocol):
    def __call__(self, messages: Optional[list] = None) -> dict[str, Any]:
        ...


def get_request(messages: list = None) -> dict:
    return  {
    "model": GET_INTENT_MODEL_NAME,
    "messages": messages,
    "temperature": 0,
    "response_format": {
        "type": "json_schema",
        "json_schema": {
            "name": "intent_decision",
            "strict": True,
            "schema": {
                "type": "object",
                "properties": {
                    "action": {"type": "string", "enum": ["tool_calls", "reply", "ask_user", "error"]},
                    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
                    "reason": {"type": "string", "minLength": 1},
                    "reply": {"type": "string"},
                    "tool_calls": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "name": {"type": "string"},
                                "arguments": {"type": "object"}
                            },
                            "required": ["name", "arguments"],
                            "additionalProperties": False
                        }
                    }
                },
                "required": ["action", "confidence", "reason"],
                "additionalProperties": False
            }
        }
    }
}

class IntentRequestBuilder(RequestBuilder):
    # 对应预算意图提取请求
    def __call__(self, messages: Optional[list] = None) -> dict[str, Any]:
        ...





def get_intent_request(messages: list = None) -> dict:
    """预算意图提取请求（与 GET_INTENT_PROMPT 对齐）。"""
    return {
        "model": GET_INTENT_MODEL_NAME,
        "messages": messages,
        "temperature": 0,
        "response_format": {
            "type": "json_schema",
            "json_schema": {
                "name": "budget_intent",
                "strict": True,
                "schema": {
                    "type": "object",
                    "properties": {
                        "has_budget_intent": {"type": "boolean"},
                        "budget_max": {"type": ["number", "null"]},
                        "budget_range": {
                            "anyOf": [
                                {
                                    "type": "object",
                                    "properties": {
                                        "min": {"type": "number"},
                                        "max": {"type": "number"},
                                    },
                                    "required": ["min", "max"],
                                    "additionalProperties": False,
                                },
                                {"type": "null"},
                            ]
                        },
                    },
                    "required": ["has_budget_intent", "budget_max", "budget_range"],
                    "additionalProperties": False,
                },
            },
        },
    }


