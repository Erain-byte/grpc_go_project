package svc

import (
	"testing"

	"gateway/internal/config"
	redisclient "gateway/internal/redis"
)

func TestNewServiceContextInjectsDependencies(t *testing.T) {
	cfg := config.Config{
		Name: "test-gateway",
		Redis: config.RedisConfig{
			Host: "redis.local",
			Port: 6380,
		},
	}
	rdb := redisclient.NewRedisClient(cfg.Redis)
	t.Cleanup(func() { _ = rdb.Close() })

	serviceContext := NewServiceContext(cfg, rdb, nil)

	if serviceContext.Config.Name != cfg.Name {
		t.Fatalf("config name = %q, want %q", serviceContext.Config.Name, cfg.Name)
	}
	if serviceContext.Redis != rdb {
		t.Fatal("Redis client was not injected into ServiceContext")
	}
	if serviceContext.Registry != nil {
		t.Fatal("Registry should remain nil when nil is injected")
	}
}
