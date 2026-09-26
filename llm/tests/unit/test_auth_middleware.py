import grpc
import pytest
import pytest_asyncio
from grpc_health.v1 import _async as health_aio
from grpc_health.v1 import health_pb2, health_pb2_grpc

from llm.v1 import llm_pb2, llm_pb2_grpc
from llm_service.handler.chat import ChatHandler
from llm_service.middleware.auth import AuthMiddleware


@pytest_asyncio.fixture
async def grpc_endpoint():
    server = grpc.aio.server(interceptors=(AuthMiddleware(),))
    llm_pb2_grpc.add_LlmServiceServicer_to_server(ChatHandler(), server)

    health_servicer = health_aio.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    await health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)

    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    try:
        yield f"127.0.0.1:{port}"
    finally:
        await server.stop(0)


@pytest.mark.asyncio
async def test_health_does_not_require_identity(grpc_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(grpc_endpoint) as channel:
        stub = health_pb2_grpc.HealthStub(channel)
        response = await stub.Check(health_pb2.HealthCheckRequest())

    assert response.status == health_pb2.HealthCheckResponse.SERVING


@pytest.mark.asyncio
async def test_chat_rejects_missing_identity(grpc_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(grpc_endpoint) as channel:
        stub = llm_pb2_grpc.LlmServiceStub(channel)
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await stub.Chat(llm_pb2.ChatRequest(message="hello"))

    assert caught.value.code() == grpc.StatusCode.UNAUTHENTICATED


@pytest.mark.asyncio
async def test_chat_rejects_missing_role(grpc_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(grpc_endpoint) as channel:
        stub = llm_pb2_grpc.LlmServiceStub(channel)
        with pytest.raises(grpc.aio.AioRpcError) as caught:
            await stub.Chat(
                llm_pb2.ChatRequest(message="hello"),
                metadata=(("x-user-id", "2"),),
            )

    assert caught.value.code() == grpc.StatusCode.UNAUTHENTICATED


@pytest.mark.asyncio
async def test_chat_uses_authenticated_context(grpc_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(grpc_endpoint) as channel:
        stub = llm_pb2_grpc.LlmServiceStub(channel)
        response = await stub.Chat(
            llm_pb2.ChatRequest(message="hello"),
            metadata=(("x-user-id", "2"), ("x-user-role", "admin")),
        )

    assert response.user_id == "2"


@pytest.mark.asyncio
async def test_stream_chat_uses_auth_middleware(grpc_endpoint: str) -> None:
    async with grpc.aio.insecure_channel(grpc_endpoint) as channel:
        stub = llm_pb2_grpc.LlmServiceStub(channel)
        call = stub.StreamChat(
            llm_pb2.StreamChatRequest(message="hello world"),
            metadata=(("x-user-id", "2"), ("x-user-role", "admin")),
        )
        responses = [response async for response in call]

    assert responses[-1].finished is True
