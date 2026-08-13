package redis

import (
	"testing"
	"time"

	"gateway/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

func TestNewRedisClientSingleNode(t *testing.T) {
	cfg := config.RedisConfig{Host: "redis.local", Port: 6380}
	client := NewRedisClient(cfg)
	t.Cleanup(func() { _ = client.Close() })

	raw, ok := client.Raw().(*goredis.Client)
	if !ok {
		t.Fatalf("expected a single-node client, got %T", client.Raw())
	}
	if got := raw.Options().Addr; got != "redis.local:6380" {
		t.Fatalf("Redis address = %q, want redis.local:6380", got)
	}
	if got := raw.Options().PoolSize; got != 100 {
		t.Fatalf("pool size = %d, want 100", got)
	}
}

func TestNewRedisClientCluster(t *testing.T) {
	addresses := []string{"redis-1:6379", "redis-2:6379"}
	cfg := config.RedisConfig{ClusterAddresses: addresses, PoolSize: 20}
	client := NewRedisClient(cfg)
	t.Cleanup(func() { _ = client.Close() })

	raw, ok := client.Raw().(*goredis.ClusterClient)
	if !ok {
		t.Fatalf("expected a cluster client, got %T", client.Raw())
	}
	if got := len(raw.Options().Addrs); got != len(addresses) {
		t.Fatalf("Redis address count = %d, want %d", got, len(addresses))
	}
	if got := raw.Options().PoolSize; got != 20 {
		t.Fatalf("pool size = %d, want 20", got)
	}
}

func TestParseDuration(t *testing.T) {
	fallback := 10 * time.Second
	if got := parseDuration("2m", fallback); got != 2*time.Minute {
		t.Fatalf("duration = %s, want 2m", got)
	}
	if got := parseDuration("invalid", fallback); got != fallback {
		t.Fatalf("duration = %s, want fallback %s", got, fallback)
	}
}
