package svc

import (
	"context"
	"fmt"
	"time"

	"gateway/internal/config"
	"gateway/internal/consul"

	"github.com/redis/go-redis/v9"
)

// ServiceContext contains the long-lived dependencies shared by the service.
type ServiceContext struct {
	Config   config.Config
	Redis    redis.UniversalClient
	Registry *consul.ConsulRegistry
}

// NewServiceContext creates the service dependencies from cfg.
func NewServiceContext(cfg config.Config) *ServiceContext {
	redisCfg := cfg.Redis
	poolSize := redisCfg.PoolSize
	if poolSize <= 0 {
		poolSize = 100
	}
	minIdleConns := redisCfg.MinIdleConns
	if minIdleConns < 0 {
		minIdleConns = 0
	}

	var rdb redis.UniversalClient
	if redisCfg.IsClusterMode() {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           redisCfg.ClusterAddresses,
			Password:        redisCfg.Password,
			PoolSize:        poolSize,
			MinIdleConns:    minIdleConns,
			MaxIdleConns:    nonNegative(redisCfg.MaxIdleConns),
			ConnMaxIdleTime: parseDuration(redisCfg.ConnMaxIdleTime, 30*time.Minute),
			ConnMaxLifetime: parseDuration(redisCfg.ConnMaxLifetime, time.Hour),
			DialTimeout:     parseDuration(redisCfg.DialTimeout, 5*time.Second),
			ReadTimeout:     parseDuration(redisCfg.ReadTimeout, 3*time.Second),
			WriteTimeout:    parseDuration(redisCfg.WriteTimeout, 3*time.Second),
			PoolTimeout:     parseDuration(redisCfg.PoolTimeout, 4*time.Second),
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:            redisCfg.GetRedisAddress(),
			Password:        redisCfg.Password,
			DB:              redisCfg.DB,
			PoolSize:        poolSize,
			MinIdleConns:    minIdleConns,
			MaxIdleConns:    nonNegative(redisCfg.MaxIdleConns),
			ConnMaxIdleTime: parseDuration(redisCfg.ConnMaxIdleTime, 30*time.Minute),
			ConnMaxLifetime: parseDuration(redisCfg.ConnMaxLifetime, time.Hour),
			DialTimeout:     parseDuration(redisCfg.DialTimeout, 5*time.Second),
			ReadTimeout:     parseDuration(redisCfg.ReadTimeout, 3*time.Second),
			WriteTimeout:    parseDuration(redisCfg.WriteTimeout, 3*time.Second),
			PoolTimeout:     parseDuration(redisCfg.PoolTimeout, 4*time.Second),
		})
	}

	return &ServiceContext{Config: cfg, Redis: rdb}
}

// HealthCheck verifies Redis connectivity when it is required by configuration.
func (svc *ServiceContext) HealthCheck(ctx context.Context) error {
	if svc == nil {
		return fmt.Errorf("service context is nil")
	}
	if !svc.Config.Redis.HealthRequired {
		return nil
	}
	if svc.Redis == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	if err := svc.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check: %w", err)
	}
	return nil
}

// Close releases resources owned by the service context.
func (svc *ServiceContext) Close() error {
	if svc == nil {
		return nil
	}
	if svc.Registry != nil {
		svc.Registry.Close()
	}
	if svc.Redis == nil {
		return nil
	}
	return svc.Redis.Close()
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
