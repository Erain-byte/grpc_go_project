// Package redis 封装 Web3 服务使用的 Redis 连接和常用原子操作。
package redis

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"web3/internal/config"
	"web3/pkg/apperror"

	goredis "github.com/redis/go-redis/v9"
)

// Client 隐藏单机与集群客户端的差异，并统一应用 web3: 键前缀。
type Client interface {
	Get(ctx context.Context, key string) (string, error)
	GetDel(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error)
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (bool, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Incr(ctx context.Context, key string) (int64, error)
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	Ping(ctx context.Context) error
	Close() error
	Raw() goredis.UniversalClient
}

type client struct {
	raw       goredis.UniversalClient
	keyPrefix string
}

var _ Client = (*client)(nil)

// NewClient 只创建连接池；是否要求 Redis 启动成功由 app 层通过 Ping 决定。
func NewClient(cfg config.RedisConfig) Client {
	var raw goredis.UniversalClient
	if len(cfg.ClusterAddresses) > 0 {
		raw = goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs:           cfg.GetAddresses(),
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxIdleTime: parseDuration(cfg.ConnMaxIdleTime),
			ConnMaxLifetime: parseDuration(cfg.ConnMaxLifetime),
			DialTimeout:     parseDuration(cfg.DialTimeout),
			ReadTimeout:     parseDuration(cfg.ReadTimeout),
			WriteTimeout:    parseDuration(cfg.WriteTimeout),
			PoolTimeout:     parseDuration(cfg.PoolTimeout),
		})
	} else {
		raw = goredis.NewClient(&goredis.Options{
			Addr:            cfg.GetAddresses()[0],
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxIdleTime: parseDuration(cfg.ConnMaxIdleTime),
			ConnMaxLifetime: parseDuration(cfg.ConnMaxLifetime),
			DialTimeout:     parseDuration(cfg.DialTimeout),
			ReadTimeout:     parseDuration(cfg.ReadTimeout),
			WriteTimeout:    parseDuration(cfg.WriteTimeout),
			PoolTimeout:     parseDuration(cfg.PoolTimeout),
		})
	}

	return &client{raw: raw, keyPrefix: cfg.KeyPrefix}
}

func parseDuration(value string) time.Duration {
	duration, _ := time.ParseDuration(value)
	return duration
}

func (c *client) key(key string) string {
	if strings.HasPrefix(key, c.keyPrefix) {
		return key
	}
	return c.keyPrefix + key
}

func (c *client) keys(keys []string) []string {
	prefixed := make([]string, len(keys))
	for i, key := range keys {
		prefixed[i] = c.key(key)
	}
	return prefixed
}

func redisError(err error, message string) error {
	// apperror.Wrap 返回 *apperror.Error。若直接把 nil 指针返回为 error 接口，
	// 接口会因为携带具体类型而不等于 nil，导致成功操作被误判为失败。
	if err == nil {
		return nil
	}
	return apperror.Wrap(err, apperror.CodeUnavailable, message, http.StatusServiceUnavailable)
}

func (c *client) Get(ctx context.Context, key string) (string, error) {
	value, err := c.raw.Get(ctx, c.key(key)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", redisError(err, "failed to get Redis value")
	}
	return value, nil
}

// GetDel 原子读取并删除一次性数据，适合消费钱包签名 Nonce。
func (c *client) GetDel(ctx context.Context, key string) (string, error) {
	value, err := c.raw.GetDel(ctx, c.key(key)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", redisError(err, "failed to consume Redis value")
	}
	return value, nil
}

func (c *client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return redisError(c.raw.Set(ctx, c.key(key), value, expiration).Err(), "failed to set Redis value")
}

func (c *client) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	created, err := c.raw.SetNX(ctx, c.key(key), value, expiration).Result()
	if err != nil {
		return false, redisError(err, "failed to create Redis value")
	}
	return created, nil
}

func (c *client) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return redisError(c.raw.Del(ctx, c.keys(keys)...).Err(), "failed to delete Redis value")
}

func (c *client) Exists(ctx context.Context, keys ...string) (bool, error) {
	if len(keys) == 0 {
		return false, nil
	}
	count, err := c.raw.Exists(ctx, c.keys(keys)...).Result()
	if err != nil {
		return false, redisError(err, "failed to check Redis value")
	}
	return count > 0, nil
}

func (c *client) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	updated, err := c.raw.Expire(ctx, c.key(key), expiration).Result()
	if err != nil {
		return false, redisError(err, "failed to update Redis expiration")
	}
	return updated, nil
}

func (c *client) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.raw.TTL(ctx, c.key(key)).Result()
	if err != nil {
		return 0, redisError(err, "failed to get Redis expiration")
	}
	return ttl, nil
}

func (c *client) Incr(ctx context.Context, key string) (int64, error) {
	value, err := c.raw.Incr(ctx, c.key(key)).Result()
	if err != nil {
		return 0, redisError(err, "failed to increment Redis value")
	}
	return value, nil
}

// Eval 会为脚本中的所有 KEYS 自动添加服务前缀，ARGV 保持原样。
func (c *client) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	result, err := c.raw.Eval(ctx, script, c.keys(keys), args...).Result()
	if err != nil {
		return nil, redisError(err, "failed to execute Redis script")
	}
	return result, nil
}

func (c *client) Ping(ctx context.Context) error {
	return redisError(c.raw.Ping(ctx).Err(), "Redis is unavailable")
}

func (c *client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return apperror.Wrap(c.raw.Close(), apperror.CodeInternal, "failed to close Redis client", http.StatusInternalServerError)
}

// Raw 仅用于 go-redis 没有被接口覆盖的高级操作；调用者需要自行添加键前缀。
func (c *client) Raw() goredis.UniversalClient { return c.raw }
