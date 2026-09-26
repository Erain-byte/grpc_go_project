import ipaddress
import logging
from urllib.parse import urlsplit
import asyncio
import consul

from llm_service.config import AppConfig

logger = logging.getLogger(__name__)


class ConsulRegistry:
    def __init__(self, cfg: AppConfig) -> None:
        self.cfg = cfg

        consul_cfg = cfg.consul
        address = urlsplit(f"//{consul_cfg.address}")
        if address.hostname is None or address.port is None:
            raise ValueError("consul.address must be host:port")

        token = consul_cfg.token.get_secret_value() or None
        self._client = consul.Consul(
            host=address.hostname,
            port=address.port,
            token=token,
            scheme=consul_cfg.scheme,
        )

    @property
    def service_id(self) -> str:
        return self.cfg.consul.service_id

    def register(self) -> None:
        consul_cfg = self.cfg.consul
        grpc_cfg = self.cfg.grpc
        check_host = consul_cfg.check_host.strip() or self.cfg.advertise_host

        check: dict[str, object] = {
            "GRPC": self._host_port(check_host, self.cfg.grpc_port),
            "GRPCUseTLS": grpc_cfg.use_tls,
            "Interval": consul_cfg.check_interval,
            "Timeout": consul_cfg.check_timeout,
            "DeregisterCriticalServiceAfter": consul_cfg.deregister_critical_after,
        }
        if grpc_cfg.use_tls:
            check["TLSServerName"] = grpc_cfg.tls_server_name
            check["TLSSkipVerify"] = consul_cfg.tls_skip_verify

        registered = self._client.agent.service.register(
            name=consul_cfg.service_name,
            service_id=self.service_id,
            address=self.cfg.advertise_host,
            port=self.cfg.grpc_port,
            tags=consul_cfg.tags,
            check=check,
        )
        if not registered:
            raise RuntimeError(f"Consul registration failed: {self.service_id}")

        logger.info("Service %s registered", self.service_id)

    def deregister(self) -> None:
        deregistered = self._client.agent.service.deregister(self.service_id)
        if not deregistered:
            raise RuntimeError(f"Consul deregistration failed: {self.service_id}")

        logger.info("Service %s deregistered", self.service_id)

    def is_registered(self) -> bool:
        services = self._client.agent.services()
        return self.service_id in services

    @staticmethod
    def _host_port(host: str, port: int) -> str:
        try:
            ip = ipaddress.ip_address(host)
        except ValueError:
            return f"{host}:{port}"

        if ip.version == 6:
            return f"[{host}]:{port}"
        return f"{host}:{port}"

