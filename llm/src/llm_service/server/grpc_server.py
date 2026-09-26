import ipaddress

import grpc
from grpc_health.v1 import _async as health_aio  # type: ignore
from grpc_health.v1 import health_pb2, health_pb2_grpc

from llm.v1 import llm_pb2_grpc
from llm_service.config import AppConfig, duration_seconds
from llm_service.handler.chat import ChatHandler
from llm_service.middleware.auth import AuthMiddleware
from llm_service.middleware.error_recovery import ErrorRecoveryMiddleware
from llm_service.middleware.logger import LoggerMiddleware

class GrpcServer:
    def __init__(self, cfg: AppConfig) -> None:
        self.cfg = cfg
        self.server: grpc.aio.Server | None = None
        self.health_servicer: health_aio.HealthServicer | None = None

    async def start(self) -> None:
        if self.server is not None:
            raise RuntimeError("gRPC server is already started")
        if self.cfg.grpc.use_tls:
            raise NotImplementedError("gRPC TLS is enabled but not implemented")
        #创建异步gRPC服务实列
        server = grpc.aio.server(
            interceptors=(
                ErrorRecoveryMiddleware(),
                LoggerMiddleware(),
                AuthMiddleware(),
            )
        )
        llm_pb2_grpc.add_LlmServiceServicer_to_server(ChatHandler(), server)#注册服务

        health_servicer = health_aio.HealthServicer()#健康检查配置
        health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)#注册健康检查

        address = self.build_grpc_addr()
        bound_port = server.add_insecure_port(address)
        if bound_port == 0:
            raise RuntimeError(f"failed to bind gRPC server to {address}")
        #启动服务
        await server.start()
        await health_servicer.set("", health_pb2.HealthCheckResponse.SERVING) #健康检查

        health_service = self.cfg.grpc.health.service.strip()
        if health_service:
            await health_servicer.set(
                health_service,
                health_pb2.HealthCheckResponse.SERVING,
            )

        self.server = server
        self.health_servicer = health_servicer

    async def wait(self) -> None:
        if self.server is None:
            raise RuntimeError("gRPC server is not started")
        await self.server.wait_for_termination() #等待服务终止

    async def stop(self) -> None:
        server = self.server
        health_servicer = self.health_servicer
        if server is None:
            return

        if health_servicer is not None:
            await health_servicer.set(
                "",
                health_pb2.HealthCheckResponse.NOT_SERVING,
            )

        await server.stop(duration_seconds(self.cfg.shutdown.timeout))#type: ignore
        self.server = None
        self.health_servicer = None

    async def run(self) -> None:
        await self.start()
        try:
            await self.wait()
        finally:
            await self.stop()

    def build_grpc_addr(self) -> str:
        host = self.cfg.host.strip()
        port = self.cfg.grpc_port

        try:
            ip = ipaddress.ip_address(host)
        except ValueError:
            return f"{host}:{port}"

        if ip.version == 6:
            return f"[{host}]:{port}"
        return f"{host}:{port}"

    async def set_health_stop(self) -> None:
        if self.health_servicer is not None:
            await self.health_servicer.set(
                "",
                health_pb2.HealthCheckResponse.NOT_SERVING,
            )
       
