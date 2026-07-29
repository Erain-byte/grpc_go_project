package svc

import (
	"context"
	"testing"
	"time"

	"gateway/internal/config"

	"github.com/redis/go-redis/v9"
)

func TestNewServiceContextSingleNode(t *testing.T) {
	cfg := config.Config{Redis: config.RedisConfig{Host: "redis.local", Port: 6380}}
	svc := NewServiceContext(cfg)
	t.Cleanup(func() { _ = svc.Close() })

	client, ok := svc.Redis.(*redis.Client)
	if !ok {
		t.Fatalf("expected a single-node client, got %T", svc.Redis)
	}
	options := client.Options()
	if options.Addr != "redis.local:6380" {
		t.Fatalf("unexpected Redis address: %s", options.Addr)
	}
	if options.PoolSize != 100 {
		t.Fatalf("expected default pool size 100, got %d", options.PoolSize)
	}
}

func TestNewServiceContextCluster(t *testing.T) {
	addresses := []string{"redis-1:6379", "redis-2:6379"}
	cfg := config.Config{Redis: config.RedisConfig{ClusterAddresses: addresses, PoolSize: 20}}
	svc := NewServiceContext(cfg)
	t.Cleanup(func() { _ = svc.Close() })

	client, ok := svc.Redis.(*redis.ClusterClient)
	if !ok {
		t.Fatalf("expected a cluster client, got %T", svc.Redis)
	}
	options := client.Options()
	if len(options.Addrs) != len(addresses) {
		t.Fatalf("unexpected Redis addresses: %v", options.Addrs)
	}
	if options.PoolSize != 20 {
		t.Fatalf("expected pool size 20, got %d", options.PoolSize)
	}
}

func TestNewServiceContextClusterWithOneAddress(t *testing.T) {
	cfg := config.Config{Redis: config.RedisConfig{ClusterAddresses: []string{"redis-1:6379"}}}
	svc := NewServiceContext(cfg)
	t.Cleanup(func() { _ = svc.Close() })

	if _, ok := svc.Redis.(*redis.ClusterClient); !ok {
		t.Fatalf("cluster_addresses must select cluster mode, got %T", svc.Redis)
	}
}

func TestHealthCheckNotRequired(t *testing.T) {
	svc := &ServiceContext{Config: config.Config{Redis: config.RedisConfig{HealthRequired: false}}}
	if err := svc.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check should be skipped: %v", err)
	}
}

func TestHealthCheckRequiredWithoutClient(t *testing.T) {
	svc := &ServiceContext{Config: config.Config{Redis: config.RedisConfig{HealthRequired: true}}}
	if err := svc.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected an error for a missing Redis client")
	}
}

func TestParseDuration(t *testing.T) {
	fallback := 10 * time.Second
	if got := parseDuration("2m", fallback); got != 2*time.Minute {
		t.Fatalf("expected 2m, got %s", got)
	}
	if got := parseDuration("invalid", fallback); got != fallback {
		t.Fatalf("expected fallback %s, got %s", fallback, got)
	}
}
