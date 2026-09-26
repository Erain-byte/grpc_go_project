import logging
import time
from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any, cast

import grpc
from llm_service.middleware.context import current_context

logger = logging.getLogger(__name__)

class LoggerMiddleware(grpc.aio.ServerInterceptor):

      async def intercept_service(
                  self,
                  continuation: Callable[
                      [grpc.HandlerCallDetails],
                      Awaitable[grpc.RpcMethodHandler | None],
                  ],
                  handler_call_details: grpc.HandlerCallDetails,
              ) -> grpc.RpcMethodHandler | None:
              handler = await continuation(handler_call_details)   
              if handler is None :
                  return handler
              method= handler_call_details.method

              if handler.unary_unary is not None:
                 unary_unary_handler = cast(
                      Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
                      handler.unary_unary,
                 )
                 return grpc.unary_unary_rpc_method_handler(
                      self._unary_unary(unary_unary_handler,method),
                      request_deserializer=handler.request_deserializer,
                      response_serializer=handler.response_serializer,
                 )

              if handler.unary_stream is not None:
                 unary_stream_handler = cast(
                      Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
                      handler.unary_stream,
                 )
                 return grpc.unary_stream_rpc_method_handler(
                      self._unary_stream(unary_stream_handler,method),
                      request_deserializer=handler.request_deserializer,
                      response_serializer=handler.response_serializer,
                 )

              return handler
      @staticmethod
      def _unary_unary(
                  handler: Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
                  method: str,
              ) -> Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]]:
              async def unary_unary(
                      request: Any,
                      context: grpc.aio.ServicerContext,
                  ) -> Any:
                   started_at =time.perf_counter()
                   try:
                        return await handler(request, context)
                   finally:
                        _write_request_log(method,context,started_at)
              return unary_unary
            

      @staticmethod
      def _unary_stream(
                handler: Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
                method: str,
            )->Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]]:
                async def unary_stream(
                            request: Any,
                            context: grpc.aio.ServicerContext,
                        ) -> AsyncIterator[Any]:
                            started_at =time.perf_counter()
                            try:
                                async for response in handler(request, context):
                                        yield response
                            finally:
                                _write_request_log(method,context,started_at)
                return unary_stream


def _write_request_log(method:str,context:grpc.aio.ServicerContext,started_at:float):
      duration = (time.perf_counter() - started_at)*1000
      code= context.code()
      if code is None:
            code=grpc.StatusCode.OK

      log_context: dict[str,object]={
            "rpc_method":method,
            "rpc_code":code,
            "duration_ms":round(duration,3)
      }
      try:
            identity=current_context()
            log_context["user_id"]=identity.user_id
            log_context["role"]=identity.role
            log_context["session_id"]=identity.session_id
      except Exception:
            pass
      if code != grpc.StatusCode.OK:
           logger.info(
                 "gRPC request completed",
                 extra=log_context,
           )
      else:
            logger.warning(
                 "gRPC request completed with error",
                 extra=log_context,
            )