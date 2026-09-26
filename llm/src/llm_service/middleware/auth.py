from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any, cast

import grpc

from llm_service.middleware import context as ctx


class AuthMiddleware(grpc.aio.ServerInterceptor):
    """校验 Gateway 通过 gRPC metadata 传递的用户身份。"""

    async def intercept_service(
        self,
        continuation: Callable[
            [grpc.HandlerCallDetails],
            Awaitable[grpc.RpcMethodHandler | None],
        ],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        handler = await continuation(handler_call_details)
        if handler is None or handler_call_details.method.startswith(
            ctx.HEALTH_METHOD_PREFIX
        ):
            return handler

        if handler.unary_unary is not None:
            unary_unary_handler = cast(
                Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
                handler.unary_unary,
            )
            return grpc.unary_unary_rpc_method_handler(
                self._unary_unary(unary_unary_handler),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        if handler.unary_stream is not None:
            unary_stream_handler = cast(
                Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
                handler.unary_stream,
            )
            return grpc.unary_stream_rpc_method_handler(
                self._unary_stream(unary_stream_handler),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        return handler

    @staticmethod
    def _unary_unary(
        handler: Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
    ) -> Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]]:
        async def unary_unary(request: Any, context: grpc.aio.ServicerContext) -> Any:
            identity = await _identity_from_context(context)
            token = ctx.set_current_context(identity)
            try:
                return await handler(request, context)
            finally:
                ctx.reset_current_context(token)

        return unary_unary

    @staticmethod
    def _unary_stream(
        handler: Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
    ) -> Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]]:
        async def unary_stream(
            request: Any,
            context: grpc.aio.ServicerContext,
        ) -> AsyncIterator[Any]:
            identity = await _identity_from_context(context)
            token = ctx.set_current_context(identity)
            try:
                async for response in handler(request, context):
                    yield response
            finally:
                ctx.reset_current_context(token)

        return unary_stream


async def _identity_from_context(context: grpc.aio.ServicerContext) -> ctx.Context:
    metadata = context.invocation_metadata()
    if not metadata:
        await context.abort(
            grpc.StatusCode.UNAUTHENTICATED,
            "metadata is missing",
        )
        raise AssertionError("context.abort returned unexpectedly")

    values = {
        key.lower(): value
        for key, value in metadata
        if isinstance(value, str)
    }

    user_id = values.get(ctx.METADATA_USER_ID, "").strip()
    if not user_id:
        await context.abort(
            grpc.StatusCode.UNAUTHENTICATED,
            "x-user-id metadata is missing",
        )

    role = values.get(ctx.METADATA_USER_ROLE, "").strip()
    if not role:
        await context.abort(
            grpc.StatusCode.UNAUTHENTICATED,
            "x-user-role metadata is missing",
        )

    return ctx.Context(
        user_id=user_id,
        role=role,
        session_id=values.get(ctx.METADATA_SESSION_ID, "").strip(),
        token_id=values.get(ctx.METADATA_TOKEN_ID, "").strip(),
    )
