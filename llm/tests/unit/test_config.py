import pytest
import yaml

from llm_service.config import ConfigError, duration_seconds, load_config


def write_config(tmp_path, data):
    path = tmp_path / "llm.yaml"
    path.write_text(yaml.safe_dump(data), encoding="utf-8")
    return path


def test_default_file_works_outside_project_directory(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    config = load_config()
    assert config.grpc_port == 9083
    assert config.consul.service_name == "llm-service-grpc"
    assert config.advertise_host != config.host


def test_token_from_environment_is_not_exposed(tmp_path, monkeypatch):
    monkeypatch.setenv("LLM_CONSUL_TOKEN", "test-only-secret")
    config = load_config(write_config(tmp_path, {"environment": "development"}))
    assert config.consul.token.get_secret_value() == "test-only-secret"
    assert "test-only-secret" not in repr(config)
    assert "token" not in config.model_dump()["consul"]


@pytest.mark.parametrize("data", [
    {"grpc_prt": 9083},
    {"grpc_port": 0},
    {"grpc_port": "9083"},
    {"model": {"timeout": "0s"}},
    {"consul": {"address": "http://127.0.0.1:8500"}},
    {"consul": {"tags": ["llm-service"]}},
    {"consul": {"service_name": "wrong-name"}},
    {"consul": {"registration_maintenance": {"max_backoff": "1s"}}},
    {"advertise_host": "0.0.0.0"},
    {"grpc": {"health": {"enabled": False}}},
    {"tracing": {"sampling_ratio": 2.0}},
    {"grpc": {"use_tls": True}},
])
def test_invalid_config_is_rejected(tmp_path, data):
    with pytest.raises(ConfigError):
        load_config(write_config(tmp_path, data))


def test_yaml_token_is_rejected_without_echoing_secret(tmp_path):
    path = write_config(tmp_path, {"consul": {"token": "sensitive-value"}})
    with pytest.raises(ConfigError) as error:
        load_config(path)
    assert "sensitive-value" not in str(error.value)


def test_missing_empty_and_malformed_files(tmp_path):
    path = tmp_path / "missing.yaml"
    with pytest.raises(ConfigError, match="cannot read"):
        load_config(path)
    path.write_text("", encoding="utf-8")
    with pytest.raises(ConfigError, match="non-empty YAML mapping"):
        load_config(path)
    path.write_text("consul: [", encoding="utf-8")
    with pytest.raises(ConfigError, match="invalid YAML"):
        load_config(path)


def test_tls_paths_resolve_relative_to_config(tmp_path):
    # 只验证路径，证书内容有效性由 gRPC/TLS 初始化验证。
    (tmp_path / "server.crt").write_text("placeholder", encoding="utf-8")
    (tmp_path / "server.key").write_text("placeholder", encoding="utf-8")
    path = write_config(tmp_path, {"grpc": {
        "use_tls": True, "cert_file": "server.crt", "key_file": "server.key",
    }})
    config = load_config(path)
    assert config.grpc.cert_file == str((tmp_path / "server.crt").resolve())
    (tmp_path / "server.key").unlink()
    with pytest.raises(ConfigError, match="key_file file does not exist"):
        load_config(path)


def test_disabled_consul_allows_standalone_server(tmp_path):
    config = load_config(write_config(tmp_path, {
        "consul": {"enabled": False}, "grpc": {"health": {"enabled": False}},
    }))
    assert not config.consul.enabled


@pytest.mark.parametrize("value, expected", [("100ms", 0.1), ("5s", 5), ("2m", 120)])
def test_duration_conversion(value, expected):
    assert duration_seconds(value) == expected
