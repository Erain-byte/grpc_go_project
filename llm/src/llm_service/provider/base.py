from collections.abc import AsyncIterator
from typing import Protocol
from llm_service.model.types import (
    ModelChunk,
    ModelRequest,
    ModelResponse,
)

class ModelProvider(Protocol):
    async def chat(
        self,
        request: ModelRequest,
    ) -> ModelResponse:
        ...

    def stream_chat(
        self,
        request: ModelRequest,
    ) -> AsyncIterator[ModelChunk]:
        ...