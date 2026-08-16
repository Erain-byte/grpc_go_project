package redis

import (
	"admin/internal/config"
	"context"
	"gateway/pkg/apperror"
	"net/http"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	SetNx(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Delete(ctx context.Context, key ...string) error
	Exists(ctx context.Context, key ...string) (bool, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	Incr(ctx context.Context, key string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
	Raw() goredis.UniversalClient
}
type redisClient struct {
	raw goredis.UniversalClient
}

// 实现redis集群模式和单机模式的初始化
func NewRedisClient(cfg config.RedisConfig) RedisClient {
	if cfg.IsClusterMode() {
		return newClusterClient(cfg)
	}
	return newSingleClient(cfg)
}

// REDIS单机模式
func newSingleClient(cfg config.RedisConfig) RedisClient {
	raw := goredis.NewClient(&goredis.Options{
		Addr:         cfg.GetRedisAddress()[0],
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     positiveOrDefault(cfg.PoolSize, 100),
		MinIdleConns: nonNegative(cfg.MinIdleConns),
		MaxIdleConns: nonNegative(cfg.MaxIdleConns),
		ConnMaxIdleTime: parseDuration(
			cfg.ConnMaxIdleTime,
			30*time.Minute,
		), // 连接最大空闲时间
		ConnMaxLifetime: parseDuration(
			cfg.ConnMaxLifetime,
			time.Hour,
		), // 连接最大生命周期
		DialTimeout: parseDuration(
			cfg.DialTimeout,
			5*time.Second,
		), // 连接超时时间
		ReadTimeout: parseDuration(
			cfg.ReadTimeout,
			3*time.Second,
		), // 读取超时时间
		WriteTimeout: parseDuration(
			cfg.WriteTimeout,
			3*time.Second,
		), // 写入超时时间
		PoolTimeout: parseDuration(
			cfg.PoolTimeout,
			5*time.Second,
		), // 连接池超时时间
	})
	return &redisClient{raw: raw}
}

// REDIS集群模式
func newClusterClient(cfg config.RedisConfig) RedisClient {
	raw := goredis.NewClusterClient(&goredis.ClusterOptions{
		Addrs:        cfg.GetRedisAddress(),
		Password:     cfg.Password,
		PoolSize:     positiveOrDefault(cfg.PoolSize, 100),
		MinIdleConns: nonNegative(cfg.MinIdleConns),
		MaxIdleConns: nonNegative(cfg.MaxIdleConns),
		ConnMaxIdleTime: parseDuration(
			cfg.ConnMaxIdleTime,
			30*time.Minute,
		),
		ConnMaxLifetime: parseDuration(
			cfg.ConnMaxLifetime,
			time.Hour,
		),
		DialTimeout: parseDuration(
			cfg.DialTimeout,
			5*time.Second,
		),
		ReadTimeout: parseDuration(
			cfg.ReadTimeout,
			3*time.Second,
		),
		WriteTimeout: parseDuration(
			cfg.WriteTimeout,
			3*time.Second,
		),
		PoolTimeout: parseDuration(
			cfg.PoolTimeout,
			4*time.Second,
		),
	})
	return &redisClient{raw: raw}
}
func parseDuration(
	value string,
	fallback time.Duration,
) time.Duration {
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}

	return duration
}
func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}

	return value
}

// 接口实现
func (c *redisClient) Get(ctx context.Context, key string) (string, error) {
	value, err := c.raw.Get(ctx, key).Result()
	if err != nil {
		return "", apperror.Wrap(err, apperror.CodeUnavailable, "failed to get value from Redis", http.StatusServiceUnavailable)
	}
	return value, nil
}

// Set
func (c *redisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if err := c.raw.Set(ctx, key, value, expiration).Err(); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to set value in Redis", http.StatusServiceUnavailable)
	}
	return nil
}

// SetNx
func (c *redisClient) SetNx(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	result, err := c.raw.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		return false, apperror.Wrap(err, apperror.CodeUnavailable, "failed to set value in Redis", http.StatusServiceUnavailable)
	}
	return result, nil
}

// Delete
func (c *redisClient) Delete(ctx context.Context, key ...string) error {
	if len(key) == 0 {
		return nil
	}
	if err := c.raw.Del(ctx, key...).Err(); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to delete value from Redis", http.StatusServiceUnavailable)
	}
	return nil
}

// Exists
func (c *redisClient) Exists(ctx context.Context, key ...string) (bool, error) {
	if len(key) == 0 {
		return false, nil
	}
	result, err := c.raw.Exists(ctx, key...).Result()
	if err != nil {
		return false, apperror.Wrap(err, apperror.CodeUnavailable, "failed to check value in Redis", http.StatusServiceUnavailable)
	}
	return result > 0, nil
}

// Expire
func (c *redisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if err := c.raw.Expire(ctx, key, expiration).Err(); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to update Redis expiration", http.StatusServiceUnavailable)
	}
	return nil
}

// TTL
func (c *redisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := c.raw.TTL(ctx, key).Result()
	if err != nil {
		return 0, apperror.Wrap(err, apperror.CodeUnavailable, "failed to get Redis expiration", http.StatusServiceUnavailable)
	}
	return result, nil
}

// Incr
func (c *redisClient) Incr(ctx context.Context, key string) (int64, error) {
	result, err := c.raw.Incr(ctx, key).Result()
	if err != nil {
		return 0, apperror.Wrap(err, apperror.CodeUnavailable, "failed to increment value in Redis", http.StatusServiceUnavailable)
	}
	return result, nil
}

// Ping
func (c *redisClient) Ping(ctx context.Context) error {
	if err := c.raw.Ping(ctx).Err(); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "Redis is unavailable", http.StatusServiceUnavailable)
	}
	return nil
}
func (c *redisClient) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	if err := c.raw.Close(); err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to close Redis client", http.StatusInternalServerError)
	}
	return nil
}
func (c *redisClient) Raw() goredis.UniversalClient {
	return c.raw
}
