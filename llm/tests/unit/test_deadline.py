import asyncio
from unittest.mock import Mock

import grpc
import pytest

from llm_service.deadline import Deadline
from llm_service.errors import ModelTimeoutError


def make_context(remaining: float | None, *, cancelled: bool = False) -> Mock:
    context = Mock(spec=grpc.aio.ServicerContext)
    context.time_remaining.return_value = remaining
    context.cancelled.return_value = cancelled
    return context


def test_uses_operation_timeout_without_rpc_deadline() -> None:
    deadline = Deadline(make_context(None), operation_timeout=30.0)

    assert deadline.remaining() is None
    assert deadline.effective_timeout() == 30.0
    assert deadline.expired() is False


def test_uses_shorter_rpc_deadline() -> None:
    deadline = Deadline(make_context(5.0), operation_timeout=30.0)

    assert deadline.effective_timeout() == 5.0


def test_uses_shorter_operation_timeout() -> None:
    deadline = Deadline(make_context(60.0), operation_timeout=30.0)

    assert deadline.effective_timeout() == 30.0


def test_negative_remaining_time_is_expired() -> None:
    deadline = Deadline(make_context(-1.0), operation_timeout=30.0)

    assert deadline.remaining() == 0.0
    assert deadline.effective_timeout() == 0.0
    assert deadline.expired() is True


def test_rejects_non_positive_operation_timeout() -> None:
    with pytest.raises(ValueError, match="operation_timeout must be positive"):
        Deadline(make_context(None), operation_timeout=0.0)


def test_exposes_rpc_cancellation_state() -> None:
    deadline = Deadline(make_context(5.0, cancelled=True), operation_timeout=30.0)

    assert deadline.cancelled() is True


@pytest.mark.asyncio
async def test_enforce_allows_operation_to_finish() -> None:
    deadline = Deadline(make_context(None), operation_timeout=1.0)

    async with deadline.enforce():
        await asyncio.sleep(0)


@pytest.mark.asyncio
async def test_enforce_converts_timeout_to_service_error() -> None:
    deadline = Deadline(make_context(None), operation_timeout=0.01)

    with pytest.raises(ModelTimeoutError):
        async with deadline.enforce():
            await asyncio.sleep(0.1)


@pytest.mark.asyncio
async def test_enforce_rejects_expired_request() -> None:
    deadline = Deadline(make_context(0.0), operation_timeout=30.0)

    with pytest.raises(ModelTimeoutError, match="request deadline exceeded"):
        async with deadline.enforce():
            await asyncio.sleep(0)


@pytest.mark.asyncio
async def test_enforce_does_not_swallow_cancellation() -> None:
    deadline = Deadline(make_context(None), operation_timeout=1.0)

    async def cancelled_operation() -> None:
        async with deadline.enforce():
            raise asyncio.CancelledError

    with pytest.raises(asyncio.CancelledError):
        await cancelled_operation()
