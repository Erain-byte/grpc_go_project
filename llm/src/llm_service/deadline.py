import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

import grpc

from llm_service.errors import ModelTimeoutError


class Deadline:
    """Manage the time budget of one downstream operation in an RPC request."""

    def __init__(
        self,
        context: grpc.aio.ServicerContext,
        operation_timeout: float,
    ) -> None:
        if operation_timeout <= 0:
            raise ValueError("operation_timeout must be positive")

        self._context = context
        self._operation_timeout = operation_timeout

    @property
    def operation_timeout(self) -> float:
        return self._operation_timeout

    def remaining(self) -> float | None:
        """Return the inbound RPC time budget, if the caller set a deadline."""
        remaining = self._context.time_remaining()
        if remaining is None:
            return None
        return max(remaining, 0.0)

    def effective_timeout(self) -> float:
        """Use the shorter value of the RPC budget and operation timeout."""
        remaining = self.remaining()
        if remaining is None:
            return self._operation_timeout
        return min(remaining, self._operation_timeout)

    def expired(self) -> bool:
        remaining = self.remaining()
        return remaining is not None and remaining <= 0

    def cancelled(self) -> bool:
        return self._context.cancelled()

    @asynccontextmanager
    async def enforce(self) -> AsyncIterator[None]:
        """Limit an awaited operation to the current effective time budget."""
        timeout = self.effective_timeout()
        if timeout <= 0:
            raise ModelTimeoutError("request deadline exceeded")

        try:
            async with asyncio.timeout(timeout):
                yield
        except asyncio.CancelledError:
            raise
        except TimeoutError as exc:
            raise ModelTimeoutError() from exc
