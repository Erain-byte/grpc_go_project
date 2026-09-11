package svc

import (
	"gateway/internal/config"
	"gateway/internal/consul"
	redisclient "gateway/internal/redis"
	"gateway/internal/runtimeconfig"
)

// ServiceContext contains the long-lived dependencies shared by the service.
type ServiceContext struct {
	Config   config.Config
	Redis    redisclient.RedisClient
	Registry *consul.ConsulRegistry
	Runtime  *runtimeconfig.Store
}

// NewServiceContext collects dependencies created by the application layer.
func NewServiceContext(
	cfg config.Config,
	redisClient redisclient.RedisClient,
	registry *consul.ConsulRegistry,
	runtimeStore *runtimeconfig.Store,
) *ServiceContext {
	return &ServiceContext{
		Config:   cfg,
		Redis:    redisClient,
		Registry: registry,
		Runtime:  runtimeStore,
	}
}
