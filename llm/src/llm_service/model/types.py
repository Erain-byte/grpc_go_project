from dataclasses import dataclass
from typing import Optional, List, Dict, Any

@dataclass(frozen=True, slots=True)
class ModelRequest:
    message: str
    conversation_id: str
    user_id: str
    model: str

@dataclass(frozen=True, slots=True)
class TokenUsage:
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0

@dataclass(frozen=True, slots=True)
class ModelResponse:
    content: str
    model: str
    usage: TokenUsage


@dataclass(frozen=True, slots=True)
class ModelChunk:
    content: str
    model: str
    finished: bool = False
    finish_reason: str = ""