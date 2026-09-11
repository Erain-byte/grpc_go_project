package config

import "testing"

func TestInitConfigLoadsAuthConfiguration(t *testing.T) {
	t.Setenv("GATEWAY_ACCESS_TOKEN_SECRET", "test-access-secret")
	t.Setenv("GATEWAY_JWT_SECRET", "")

	cfg, err := InitConfig("../../etc/gateway.yaml")
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if got := cfg.Auth.AccessToken.Secret; got != "test-access-secret" {
		t.Fatalf("access token secret = %q, want environment value", got)
	}
	if got := cfg.Auth.AccessToken.Algorithm; got != "HS256" {
		t.Errorf("access token algorithm = %q, want HS256", got)
	}
	if got := cfg.Auth.AccessToken.Expire; got != "15m" {
		t.Errorf("access token expire = %q, want 15m", got)
	}
	if got := len(cfg.Auth.AccessToken.Issuers); got != 2 {
		t.Errorf("access token issuer count = %d, want 2", got)
	}
	if !cfg.Auth.RefreshToken.Rotate {
		t.Error("refresh token rotation should be enabled")
	}
	if !cfg.Auth.RefreshToken.ReuseDetection {
		t.Error("refresh token reuse detection should be enabled")
	}
	if got := cfg.Consul.CheckInterval; got != "10s" {
		t.Errorf("Consul check interval = %q, want 10s", got)
	}
	if got := cfg.Consul.CheckTimeout; got != "5s" {
		t.Errorf("Consul check timeout = %q, want 5s", got)
	}
	if got := cfg.Redis.MinIdleConns; got != 10 {
		t.Errorf("Redis min idle connections = %d, want 10", got)
	}
	if got := cfg.Tracing.ServiceName; got != "gateway-service" {
		t.Errorf("tracing service name = %q, want gateway-service", got)
	}
	if got := cfg.Consul.RuntimeConfigKey; got != "grpc-go/config/gateway/runtime" {
		t.Errorf("Consul runtime config key = %q", got)
	}
	if !cfg.Consul.RuntimeConfigRequired {
		t.Error("Consul runtime config should be required")
	}
	if got := len(cfg.Cors.AllowOrigins); got != 0 {
		t.Errorf("local CORS config should be empty, got %d origins", got)
	}
}

func TestInitConfigSupportsLegacyJWTSecretEnvironment(t *testing.T) {
	t.Setenv("GATEWAY_ACCESS_TOKEN_SECRET", "")
	t.Setenv("GATEWAY_JWT_SECRET", "legacy-secret")

	cfg, err := InitConfig("../../etc/gateway.yaml")
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if got := cfg.Auth.AccessToken.Secret; got != "legacy-secret" {
		t.Fatalf("access token secret = %q, want legacy environment value", got)
	}
}
