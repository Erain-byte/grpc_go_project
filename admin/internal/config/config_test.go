package config

import "testing"

func TestInitConfig(t *testing.T) {
	t.Setenv("ADMIN_DB_PASSWORD", "database-secret")
	t.Setenv("ADMIN_REDIS_PASSWORD", "redis-secret")
	t.Setenv("ADMIN_CONSUL_TOKEN", "consul-secret")
	t.Setenv("ADMIN_ACCESS_TOKEN_SECRET", "access-token-secret")

	cfg, err := InitConfig("../../etc/admin.yaml")
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}

	if cfg.Name != "admin-service" {
		t.Fatalf("name = %q, want admin-service", cfg.Name)
	}
	if cfg.Database.DBName != "admin_db" {
		t.Fatalf("database name = %q, want admin_db", cfg.Database.DBName)
	}
	if cfg.Database.Password != "database-secret" {
		t.Fatal("database password was not loaded from the environment")
	}
	if cfg.Database.ReadWriteSplitEnabled {
		t.Fatal("database read/write splitting should be disabled by default")
	}
	if len(cfg.Database.ReplicaAddresses) != 0 {
		t.Fatalf("database replica addresses = %v, want empty", cfg.Database.ReplicaAddresses)
	}
	if cfg.Redis.Password != "redis-secret" {
		t.Fatal("Redis password was not loaded from the environment")
	}
	if cfg.Consul.Token != "consul-secret" {
		t.Fatal("Consul token was not loaded from the environment")
	}
	if cfg.Auth.AccessToken.Secret != "access-token-secret" {
		t.Fatal("access-token secret was not loaded from the environment")
	}
	if cfg.Auth.AccessToken.Issuer != "admin-service" {
		t.Fatalf("issuer = %q, want admin-service", cfg.Auth.AccessToken.Issuer)
	}
	if cfg.Tracing.ServiceName != "admin-service" {
		t.Fatalf("tracing service name = %q, want admin-service", cfg.Tracing.ServiceName)
	}
	if cfg.Redis.Address() != "127.0.0.1:6379" {
		t.Fatalf("Redis address = %q, want 127.0.0.1:6379", cfg.Redis.Address())
	}
}

func TestValidateRequiresReplicaWhenReadWriteSplitEnabled(t *testing.T) {
	cfg := Config{
		Name:     "admin-service",
		Host:     "127.0.0.1",
		GRPCPort: 9081,
		Database: DatabaseConfig{ReadWriteSplitEnabled: true},
		Auth: AuthConfig{AccessToken: AccessTokenConfig{
			Issuer:   "admin-service",
			Audience: "gateway",
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing database replica error")
	}
}

func TestValidateAcceptsReadWriteSplitReplica(t *testing.T) {
	cfg := Config{
		Name:     "admin-service",
		Host:     "127.0.0.1",
		GRPCPort: 9081,
		Database: DatabaseConfig{
			ReadWriteSplitEnabled: true,
			ReplicaAddresses:      []string{"127.0.0.1:3307"},
		},
		Auth: AuthConfig{AccessToken: AccessTokenConfig{
			Issuer:   "admin-service",
			Audience: "gateway",
		}},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestInitConfigEnvironmentOverride(t *testing.T) {
	t.Setenv("ADMIN_ENVIRONMENT", "production")

	cfg, err := InitConfig("../../etc/admin.yaml")
	if err != nil {
		t.Fatalf("InitConfig() error = %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("environment override should enable production mode")
	}
}

func TestValidateRequiresTLSKeyPair(t *testing.T) {
	cfg := Config{
		Name:     "admin-service",
		Host:     "127.0.0.1",
		GRPCPort: 9081,
		Auth: AuthConfig{AccessToken: AccessTokenConfig{
			Issuer:   "admin-service",
			Audience: "gateway",
		}},
		GRPC: GRPCServerConfig{UseTLS: true},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing TLS key-pair error")
	}
}
