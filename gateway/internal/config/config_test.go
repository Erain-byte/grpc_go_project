package config

import "testing"

func TestInitConfigLoadsAuthConfiguration(t *testing.T) {
	cfg, err := InitConfig("../../etc/gateway.yaml")
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if got := cfg.Auth.AccessToken.Algorithm; got != "RS256" {
		t.Errorf("access token algorithm = %q, want RS256", got)
	}
	if got := cfg.Auth.AccessToken.Audience; got != "grpc-api" {
		t.Errorf("access token audience = %q, want grpc-api", got)
	}
	if got := len(cfg.Auth.AccessToken.Issuers); got != 1 {
		t.Errorf("access token issuer count = %d, want 1", got)
	}
	if got := cfg.Auth.AccessToken.JWKSURL; got != "http://localhost:8180/realms/grpc-project/protocol/openid-connect/certs" {
		t.Errorf("JWKS URL = %q", got)
	}
	if got := len(cfg.Auth.AccessToken.RolePriority); got != 3 {
		t.Errorf("role priority count = %d, want 3", got)
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

func TestAccessTokenConfigRolesMatching(t *testing.T) {
	cfg := AccessTokenConfig{RolePriority: []string{"super-admin", "admin", "user"}}

	adminRoles := cfg.RolesMatching("admin")
	if len(adminRoles) != 2 || adminRoles[0] != "super-admin" || adminRoles[1] != "admin" {
		t.Fatalf("admin roles = %v", adminRoles)
	}

	userRoles := cfg.RolesMatching("user")
	if len(userRoles) != 1 || userRoles[0] != "user" {
		t.Fatalf("user roles = %v", userRoles)
	}

	if roles := cfg.RolesMatching(""); roles != nil {
		t.Fatalf("empty role name returned %v", roles)
	}
}
