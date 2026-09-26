#OpenTelemetry 拦截器：接收 Gateway Trace Context 并创建服务端 Span
import grpc
from opentelemetry import trace
from opentelemetry.propagate import extract
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.trace import SpanKind


from llm_service.middleware import context as ctx
from llm_service.config import AppConfig
from collections.abc import AsyncIterator, Awaitable, Callable
from typing import Any,cast 



class TraceInterceptorMiddleware(grpc.aio.ServerInterceptor):

        async def intercept_service(
                self,
                continuation: Callable[
                    [grpc.HandlerCallDetails],
                    Awaitable[grpc.RpcMethodHandler | None],
                ],
                handler_call_details: grpc.HandlerCallDetails,
            ) -> grpc.RpcMethodHandler | None:
            handler = await continuation(handler_call_details)
            if handler is None or handler_call_details.method.startswith(ctx.HEALTH_METHOD_PREFIX):
                return handler
            
            method = handler_call_details.method  
            method_data:dict[str,str] = {
                 key.lower():value for key,value in handler_call_details.invocation_metadata if isinstance(value,str)
            }
            if handler.unary_unary is not None:
               unary_unary_handler= cast(
                    Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],
                    handler.unary_unary,
               )
               return grpc.unary_unary_rpc_method_handler(
                    self._unary_unary(unary_unary_handler,method,method_data),
                    request_deserializer=handler.request_deserializer,
                    response_serializer=handler.response_serializer,
               )
            if handler.unary_stream is not None:
                unary_stream_handler= cast(
                    Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],
                    handler.unary_stream,
                )
                return grpc.unary_stream_rpc_method_handler(
                    self._unary_stream(unary_stream_handler,method,method_data),
                    request_deserializer=handler.request_deserializer,
                    response_serializer=handler.response_serializer,
                )

            return handler

        @staticmethod
        def _unary_unary(handler: Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]],method:str,method_data:dict[str,str]) -> Callable[[Any, grpc.aio.ServicerContext], Awaitable[Any]]:
            async def unary_unary(request: Any, context: grpc.aio.ServicerContext) -> Any:
               
                   parent_context= extract(method_data)#获取Trace Context
                   tracer=_init_otel(AppConfig())
                   if tracer is None:
                       return await handler(request, context)
                   
                   with tracer.start_as_current_span(method,parent_context,kind=SpanKind.SERVER) as span:
                            span.set_attributes(
                                    {
                                    "rpc.system": "grpc",
                                    "rpc.method": method,
                                    "rpc.type": "unary_stream",
                                    }
                                )
                            return await handler(request, context)
              
            return unary_unary            
                         
        @staticmethod
        def _unary_stream(handler: Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]],method:str,method_data:dict[str,str]) -> Callable[[Any, grpc.aio.ServicerContext], AsyncIterator[Any]]:
             async def unary_stream(request: Any, context: grpc.aio.ServicerContext) -> AsyncIterator[Any]:
                   parent_context= extract(method_data)
                   tracer=_init_otel(AppConfig())
                   if tracer is None:
                        async for item in handler(request, context):
                            yield item
                        return
                   with tracer.start_as_current_span(method,parent_context,kind=SpanKind.SERVER) as span:
                            span.set_attributes(
                                {
                                "rpc.system": "grpc",
                                "rpc.method": method,
                                "rpc.type": "unary_stream",
                                }
                            )
                            async for item in handler(request, context):
                                yield item
             return unary_stream



   
#初始化otel
def _init_otel(cfg:AppConfig)->trace.Tracer| None:
    if not cfg.tracing.enabled:
        return None
    trace.set_tracer_provider(TracerProvider(resource=Resource.create({"service.name": cfg.service_name})))
    
    return trace.get_tracer(__name__)