from collections.abc import AsyncIterator

import grpc
import pytest
import pytest_asyncio

from llm.v1 import llm_pb2, llm_pb2_grpc
from llm_service.errors import (
    ConflictError,
    DataLossError,
    FailedPreconditionError,
    InvalidArgumentError,
    ModelTimeoutError,
    PermissionDeniedError,
    ResourceAlreadyExistsError,
    ResourceExhaustedError,
    ResourceNotFoundError,
    ServiceUnavailableError,
    UnauthenticatedError,
)
from llm_service.middleware.auth import AuthMiddleware
from llm_service.middleware.error_recovery import (
    ErrorRecoveryMiddleware,
    map_exception,
)

IDENTITY = (("x-user-id", "2"), ("x-user-role", "admin"))


class FailingChatHandler(llm_pb2_grpc.LlmServiceServicer):
    async def Chat(
        self,
        request: llm_pb2.ChatRequest,
        context: grpc.aio.ServicerContext,
    ) -> llm_pb2.ChatResponse:
        if request.message == "service-error":
            raise InvalidArgumentError("prompt is invalid")
        if request.message == "unknown-error":
            raise RuntimeError("private implementation detail")
        if request.message == "grpc-abort":
            await context.abort(grpc.StatusCode.NOT_FOUND, "explicit grpc error")

        return llm_pb2.ChatResponse(reply="ok", user_id="2")

    async def StreamChat(
        self,
        request: llm_pb2.StreamChatRequest,
        context: grpc.aio.ServicerContext,
    ) -> AsyncIterator[llm_pb2.StreamChatResponse]:
        yield llm_pb2.StreamChatResponse(delta="first", finished=False)
        if request.message == "stream-error":
            raise ServiceUnavailableError()
        yield llm_pb2.StreamChatResponse(finished=True)


@pytest_asyncio.fixture
async def recovery_endpoint():
    server = grpc.aio.server(
        interceptors=(
            AuthMiddleware(),
            ErrorRecoveryMiddleware(),
        )
    )
    llm_pb2_grpc.add_LlmServiceServicer_to_server(FailingChatHandler(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    try:
        yield f"127.0.0.1:{port}"
    finally:
        await server.stop(0)


@pytest.mark.parametrize(
    ("error", "expected_code"),
    [
        (InvalidArgumentError(), grpc.StatusCode.INVALID_ARGUMENT),
        (ResourceNotFoundError(), grpc.StatusCode.NOT_FOUND),
        (ResourceAlreadyExistsError(), grpc.StatusCode.ALREADY_EXISTS),
        (UnauthenticatedError(), grpc.StatusCode.UNAUTHENTICATED),
        (PermissionDeniedError(), grpc.StatusCode.PERMISSION_DENIED),
        (FailedPreconditionError(), grpc.StatusCode.FAILED_PRECONDITION),
        (ConflictError(), grpc.StatusCode.ABORTED),
        (ResourceExhaustedError(), grpc.StatusCode.RESOURCE_EXHAUSTED),
        (ModelTimeoutError(), grpc.StatusCode.DEADLINE_EXCEEDED),
        (ServiceUnavailableError(), grpc.StatusCode.UNAVAILABLE),
        (DataLossError(), grpc.StatusCode.DATA_LOSS),
    ],
)
def test_service_error_mapping(error: Exception, expected_code: grpc.StatusCode) -> None:
    code, message = map_exception(error)

    assert code == expected_code
    assert message


@pytest.mark.asyncio
async def test_unary_success_is_unchanged(recovery_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        response = await llm_pb2_grpc.LlmServiceStub(channel).Chat(
            llm_pb2.ChatRequest(message="ok"),
            metadata=IDENTITY,
        )

    assert response.reply == "ok"


@pytest.mark.asyncio
async def test_service_error_is_returned_as_public_grpc_error(
    recovery_endpoint: str,
) -> None:
    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await llm_pb2_grpc.LlmServiceStub(channel).Chat(
                llm_pb2.ChatRequest(message="service-error"),
                metadata=IDENTITY,
            )

    assert caught.value.code() == grpc.StatusCode.INVALID_ARGUMENT
    assert caught.value.details() == "prompt is invalid"


@pytest.mark.asyncio
async def test_unknown_error_does_not_leak_internal_details(
    recovery_endpoint: str,
) -> None:
    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await llm_pb2_grpc.LlmServiceStub(channel).Chat(
                llm_pb2.ChatRequest(message="unknown-error"),
                metadata=IDENTITY,
            )

    assert caught.value.code() == grpc.StatusCode.INTERNAL
    assert caught.value.details() == "internal server error"
    assert "private implementation detail" not in caught.value.details()


@pytest.mark.asyncio
async def test_explicit_grpc_abort_is_preserved(recovery_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await llm_pb2_grpc.LlmServiceStub(channel).Chat(
                llm_pb2.ChatRequest(message="grpc-abort"),
                metadata=IDENTITY,
            )

    assert caught.value.code() == grpc.StatusCode.NOT_FOUND
    assert caught.value.details() == "explicit grpc error"


@pytest.mark.asyncio
async def test_stream_keeps_previous_responses_then_returns_error(
    recovery_endpoint: str,
) -> None:
    received: list[llm_pb2.StreamChatResponse] = []

    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        call = llm_pb2_grpc.LlmServiceStub(channel).StreamChat(
            llm_pb2.StreamChatRequest(message="stream-error"),
            metadata=IDENTITY,
        )
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            async for response in call:
                received.append(response)

    assert [response.delta for response in received] == ["first"]
    assert caught.value.code() == grpc.StatusCode.UNAVAILABLE


@pytest.mark.asyncio
async def test_missing_identity_is_still_unauthenticated(recovery_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(recovery_endpoint) as channel:
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await llm_pb2_grpc.LlmServiceStub(channel).Chat(
                llm_pb2.ChatRequest(message="ok")
            )

    assert caught.value.code() == grpc.StatusCode.UNAUTHENTICATED
