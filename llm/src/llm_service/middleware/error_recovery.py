import asyncio
import logging
from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any, cast

import grpc

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
    ServiceError,
    ServiceUnavailableError,
    UnauthenticatedError,
)
from llm_service.middleware.context import current_context

logger = logging.getLogger(__name__)

ErrorResult = tuple[grpc.StatusCode, str]

_SERVICE_ERROR_CODES: dict[type[ServiceError], grpc.StatusCode] = {
    InvalidArgumentError: grpc.StatusCode.INVALID_ARGUMENT,
    ResourceNotFoundError: grpc.StatusCode.NOT_FOUND,
    ResourceAlreadyExistsError: grpc.StatusCode.ALREADY_EXISTS,
    UnauthenticatedError: grpc.StatusCode.UNAUTHENTICATED,
    PermissionDeniedError: grpc.StatusCode.PERMISSION_DENIED,
    FailedPreconditionError: grpc.StatusCode.FAILED_PRECONDITION,
    ConflictError: grpc.StatusCode.ABORTED,
    ResourceExhaustedError: grpc.StatusCode.RESOURCE_EXHAUSTED,
    ModelTimeoutError: grpc.StatusCode.DEADLINE_EXCEEDED,
    ServiceUnavailableError: grpc.StatusCode.UNAVAILABLE,
    DataLossError: grpc.StatusCode.DATA_LOSS,
}


def map_exception(exc: Exception) -> ErrorResult:
    """Map a Python exception to a public gRPC status and message."""
    if isinstance(exc, ServiceError):
        for error_type, status_code in _SERVICE_ERROR_CODES.items():
            if isinstance(exc, error_type):
                return status_code, exc.public_message
        return grpc.StatusCode.UNKNOWN, exc.public_message

    if isinstance(exc, ValueError):
        return grpc.StatusCode.INVALID_ARGUMENT, "invalid request argument"
    if isinstance(exc, PermissionError):
        return grpc.StatusCode.PERMISSION_DENIED, "permission denied"
    if isinstance(exc, LookupError):
        return grpc.StatusCode.NOT_FOUND, "resource was not found"
    if isinstance(exc, (TimeoutError, asyncio.TimeoutError)):
        return grpc.StatusCode.DEADLINE_EXCEEDED, "request timed out"

    return grpc.StatusCode.INTERNAL, "internal server error"


class ErrorRecoveryMiddleware(grpc.aio.ServerInterceptor):
    """Convert unhandled service exceptions into stable gRPC errors."""

    async def intercept_service(
        self,
        continuation: Callable[
            [grpc.HandlerCallDetails],
            Awaitable[grpc.RpcMethodHandler | None],
        ],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        handler = await continuation(handler_call_details)
        if handler is None:
            return None

        method = handler_call_details.method
        if handler.unary_unary is not None:
            unary_unary_handler = cast(
                Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
                handler.unary_unary,
            )
            return grpc.unary_unary_rpc_method_handler(
                self._unary_unary(unary_unary_handler, method),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        if handler.unary_stream is not None:
            unary_stream_handler = cast(
                Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
                handler.unary_stream,
            )
            return grpc.unary_stream_rpc_method_handler(
                self._unary_stream(unary_stream_handler, method),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        if handler.stream_unary is not None:
            stream_unary_handler = cast(
                Callable[
                    [AsyncIterator[Any], grpc.aio.ServicerContext],
                    Awaitable[Any],
                ],
                handler.stream_unary,
            )
            return grpc.stream_unary_rpc_method_handler(
                self._stream_unary(stream_unary_handler, method),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        if handler.stream_stream is not None:
            stream_stream_handler = cast(
                Callable[
                    [AsyncIterator[Any], grpc.aio.ServicerContext],
                    AsyncIterator[Any],
                ],
                handler.stream_stream,
            )
            return grpc.stream_stream_rpc_method_handler(
                self._stream_stream(stream_stream_handler, method),
                request_deserializer=handler.request_deserializer,
                response_serializer=handler.response_serializer,
            )

        return handler

    @staticmethod
    def _unary_unary(
        handler: Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
        method: str,
    ) -> Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]]:
        async def unary_unary(request: Any, context: grpc.aio.ServicerContext) -> Any:
            try:
                return await handler(request, context)
            except asyncio.CancelledError:
                raise
            except Exception as exc:  # noqa: BLE001 - RPC recovery boundary
                await _recover(context, method, exc)
                raise AssertionError("context.abort returned unexpectedly")

        return unary_unary

    @staticmethod
    def _unary_stream(
        handler: Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
        method: str,
    ) -> Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]]:
        async def unary_stream(
            request: Any,
            context: grpc.aio.ServicerContext,
        ) -> AsyncIterator[Any]:
            try:
                async for response in handler(request, context):
                    yield response
            except asyncio.CancelledError:
                raise
            except Exception as exc:  # noqa: BLE001 - RPC recovery boundary
                await _recover(context, method, exc)
                raise AssertionError("context.abort returned unexpectedly")

        return unary_stream

    @staticmethod
    def _stream_unary(
        handler: Callable[
            [AsyncIterator[Any], grpc.aio.ServicerContext],
            Awaitable[Any],
        ],
        method: str,
    ) -> Callable[
        [AsyncIterator[Any], grpc.aio.ServicerContext],
        Awaitable[Any],
    ]:
        async def stream_unary(
            request_iterator: AsyncIterator[Any],
            context: grpc.aio.ServicerContext,
        ) -> Any:
            try:
                return await handler(request_iterator, context)
            except asyncio.CancelledError:
                raise
            except Exception as exc:  # noqa: BLE001 - RPC recovery boundary
                await _recover(context, method, exc)
                raise AssertionError("context.abort returned unexpectedly")

        return stream_unary

    @staticmethod
    def _stream_stream(
        handler: Callable[
            [AsyncIterator[Any], grpc.aio.ServicerContext],
            AsyncIterator[Any],
        ],
        method: str,
    ) -> Callable[
        [AsyncIterator[Any], grpc.aio.ServicerContext],
        AsyncIterator[Any],
    ]:
        async def stream_stream(
            request_iterator: AsyncIterator[Any],
            context: grpc.aio.ServicerContext,
        ) -> AsyncIterator[Any]:
            try:
                async for response in handler(request_iterator, context):
                    yield response
            except asyncio.CancelledError:
                raise
            except Exception as exc:  # noqa: BLE001 - RPC recovery boundary
                await _recover(context, method, exc)
                raise AssertionError("context.abort returned unexpectedly")

        return stream_stream


async def _recover(
    context: grpc.aio.ServicerContext,
    method: str,
    exc: Exception,
) -> None:
    # context.abort() raises an internal AbortError. If a handler has already
    # chosen a gRPC status, preserve it instead of replacing it with INTERNAL.
    if context.code() is not None:
        raise exc

    status_code, public_message = map_exception(exc)
    log_context: dict[str, str] = {
        "rpc_method": method,
        "grpc_code": status_code.name,
        "exception_type": type(exc).__name__,
    }
    try:
        log_context["user_id"] = current_context().user_id
    except RuntimeError:
        pass

    if isinstance(exc, ServiceError):
        logger.warning("gRPC request failed", extra=log_context)
    else:
        logger.exception("unhandled gRPC request error", extra=log_context)

    await context.abort(status_code, public_message)
