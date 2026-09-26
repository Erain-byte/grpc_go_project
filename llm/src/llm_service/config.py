"""加载启动 YAML；时间字段保留字符串，调用 duration_seconds 转为秒。"""

import os
import re
from pathlib import Path
from typing import Annotated, Literal
from urllib.parse import urlsplit

import yaml
from pydantic import (
    AfterValidator,
    BaseModel,
    ConfigDict,
    Field,
    SecretStr,
    ValidationError,
    model_validator,
)

DEFAULT_CONFIG_PATH = Path(__file__).resolve().parents[2] / "config" / "llm.yaml"


class ConfigError(ValueError):
    """启动配置无法读取或不满足约束。"""


def duration_seconds(value: str) -> float:
    """将 100ms、5s、2m、1h 这样的单单位正数时长转换为秒。"""
    match = re.fullmatch(r"(\d+(?:\.\d+)?)(ms|s|m|h)", value)
    if match is None:
        raise ValueError("duration must use a positive number with ms, s, m or h")
    seconds = float(match[1]) * {"ms": 0.001, "s": 1, "m": 60, "h": 3600}[match[2]]
    if not 0 < seconds < float("inf"):
        raise ValueError("duration must be positive and finite")
    return seconds


def validate_duration(value: str) -> str:
    duration_seconds(value)
    return value


def validate_host(value: str) -> str:
    if not value or value != value.strip() or any(c.isspace() for c in value):
        raise ValueError("host must be non-empty and contain no whitespace")
    if any(c in value for c in "/?#"):
        raise ValueError("host must not contain a URL scheme, path or query")
    return value


def validate_address(value: str) -> str:
    """要求 host:port；IPv6 使用 [::1]:8500。"""
    try:
        address = urlsplit("//" + value)
        valid = (
            address.hostname
            and address.port is not None
            and 1 <= address.port <= 65535
            and address.username is None
            and not address.path
            and not address.query
            and not address.fragment
            and not any(c.isspace() for c in value)
        )
    except ValueError:
        valid = False
    if not valid:
        raise ValueError("address must be host:port without a scheme or path")
    return value


Duration = Annotated[str, AfterValidator(validate_duration)]
Host = Annotated[str, AfterValidator(validate_host)]
Address = Annotated[str, AfterValidator(validate_address)]
NonEmpty = Annotated[str, Field(min_length=1, pattern=r"\S")]


class ConfigModel(BaseModel):
    # 拼错字段或值类型时立即报错，避免错误配置悄悄被忽略。
    model_config = ConfigDict(extra="forbid", strict=True, validate_default=True)


class HealthConfig(ConfigModel):
    enabled: bool = True
    service: str = ""


class GrpcConfig(ConfigModel):
    use_tls: bool = False
    cert_file: str = ""
    key_file: str = ""
    client_ca_file: str = ""
    tls_server_name: NonEmpty = "llm-service"
    health: HealthConfig = Field(default_factory=HealthConfig)

    @model_validator(mode="after")
    def validate_tls(self) -> "GrpcConfig":
        if self.use_tls and (not self.cert_file.strip() or not self.key_file.strip()):
            raise ValueError("grpc TLS requires cert_file and key_file")
        return self


class ModelConfig(ConfigModel):
    provider: NonEmpty = "mock"
    default_model: NonEmpty = "mock-chat"
    timeout: Duration = "30s"


class RegistrationMaintenanceConfig(ConfigModel):
    enabled: bool = True
    interval: Duration = "30s"
    max_backoff: Duration = "2m"

    @model_validator(mode="after")
    def validate_backoff(self) -> "RegistrationMaintenanceConfig":
        if duration_seconds(self.max_backoff) < duration_seconds(self.interval):
            raise ValueError("max_backoff must be at least interval")
        return self


class ConsulConfig(ConfigModel):
    enabled: bool = True
    address: Address = "127.0.0.1:8500"
    scheme: Literal["http", "https"] = "http"
    token: SecretStr = Field(default_factory=lambda: SecretStr(""), repr=False, exclude=True)
    service_name: NonEmpty = "llm-service-grpc"
    service_id: NonEmpty = "llm-service-grpc-127.0.0.1-9083"
    tags: list[NonEmpty] = Field(default_factory=lambda: ["llm-service", "grpc"])
    request_timeout: Duration = "5s"
    check_host: Host = "host.docker.internal"
    check_interval: Duration = "10s"
    check_timeout: Duration = "5s"
    deregister_critical_after: Duration = "90s"
    tls_skip_verify: bool = False
    registration_maintenance: RegistrationMaintenanceConfig = Field(
        default_factory=RegistrationMaintenanceConfig
    )
    registration_required: bool = True
    deregister_on_shutdown: bool = True
   


class TracingConfig(ConfigModel):
    enabled: bool = True
    endpoint: Address = "127.0.0.1:4317"
    sampling_ratio: float = Field(default=1.0, ge=0, le=1, allow_inf_nan=False)


class LoggingConfig(ConfigModel):
    level: Literal["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"] = "INFO"
    format: Literal["json", "text"] = "json"


class ShutdownConfig(ConfigModel):
    timeout: Duration = "10s"


class AppConfig(ConfigModel):
    environment: Literal["development", "staging", "production"] = "development"
    service_name: NonEmpty = "llm-service"
    version: NonEmpty = "1.0.0"
    host: Host = "0.0.0.0"
    grpc_port: int = Field(default=9083, ge=1, le=65535)
    advertise_host: Host = "127.0.0.1"
    grpc: GrpcConfig = Field(default_factory=GrpcConfig)
    model: ModelConfig = Field(default_factory=ModelConfig)
    consul: ConsulConfig = Field(default_factory=ConsulConfig)
    tracing: TracingConfig = Field(default_factory=TracingConfig)
    logging: LoggingConfig = Field(default_factory=LoggingConfig)
    shutdown: ShutdownConfig = Field(default_factory=ShutdownConfig)

    @model_validator(mode="after")
    def validate_registration(self) -> "AppConfig":
        if self.consul.enabled:
            if self.advertise_host in {"0.0.0.0", "::", "[::]"}:
                raise ValueError("advertise_host must be a reachable address, not a wildcard")
            if not self.grpc.health.enabled:
                raise ValueError("Consul registration requires grpc.health.enabled")
            if "grpc" not in self.consul.tags:
                raise ValueError("consul.tags must include grpc for Gateway discovery")
            if self.consul.service_name != f"{self.service_name}-grpc":
                raise ValueError("consul.service_name must equal service_name + '-grpc'")
        return self


def load_config(path: str | Path | None = None) -> AppConfig:
    """读取启动配置；默认路径不依赖终端所在目录，不自动加载 .env 文件。"""
    config_path = Path(path).resolve() if path is not None else DEFAULT_CONFIG_PATH
    try:
        data = yaml.safe_load(config_path.read_text(encoding="utf-8-sig"))
    except OSError as exc:
        raise ConfigError(f"cannot read config {config_path}: {exc.strerror}") from exc
    except yaml.YAMLError as exc:
        # 不把 YAML 原文放入错误信息，避免泄露意外写入的密钥。
        raise ConfigError(f"invalid YAML in {config_path}") from exc

    if not isinstance(data, dict) or not data:
        raise ConfigError(f"config must be a non-empty YAML mapping: {config_path}")

    consul_data = data.get("consul", {})
    if isinstance(consul_data, dict):
        if "token" in consul_data:
            raise ConfigError("set LLM_CONSUL_TOKEN in the environment instead of YAML")
        # 在校验之前注入环境变量；没有启用 ACL 时允许为空。
        consul_data["token"] = os.getenv("LLM_CONSUL_TOKEN", "")
        data["consul"] = consul_data

    try:
        config = AppConfig.model_validate(data)
    except ValidationError as exc:
        details = "; ".join(
            f"{'.'.join(str(part) for part in error['loc']) or 'config'}: {error['msg']}"
            for error in exc.errors(include_input=False, include_url=False)
        )
        raise ConfigError(f"invalid config {config_path}: {details}") from exc

    if config.grpc.use_tls:
        for field in ("cert_file", "key_file", "client_ca_file"):
            value = getattr(config.grpc, field)
            if not value:
                continue
            certificate_path = Path(value)
            if not certificate_path.is_absolute():
                certificate_path = config_path.parent / certificate_path
            if not certificate_path.is_file():
                raise ConfigError(f"grpc.{field} file does not exist: {certificate_path}")
            setattr(config.grpc, field, str(certificate_path.resolve()))

    return config
