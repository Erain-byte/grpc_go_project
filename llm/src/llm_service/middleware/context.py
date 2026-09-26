from contextvars import ContextVar, Token
from dataclasses import dataclass

METADATA_USER_ID = "x-user-id"
METADATA_USER_ROLE = "x-user-role"
METADATA_SESSION_ID = "x-session-id"
METADATA_TOKEN_ID = "x-token-id"
HEALTH_METHOD_PREFIX = "/grpc.health.v1.Health/"
@dataclass
class Context:
    """
    Context for the LLM service.
    """
    user_id: str
    role: str
    session_id: str=""
    token_id: str=""

_current_context: ContextVar[Context|None] = ContextVar("current_context", default=None)

def current_context() -> Context:
    """
    Get the current context.
    """
    context = _current_context.get()
    if context is None:
        raise RuntimeError("No context set")
    return context


def set_current_context(context: Context) -> Token[Context | None]:
    return _current_context.set(context)


def reset_current_context(token: Token[Context | None]) -> None:
    _current_context.reset(token)
