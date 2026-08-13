package svc

import (
	"gateway/internal/config"
	"gateway/internal/consul"
	redisclient "gateway/internal/redis"
)

// ServiceContext contains the long-lived dependencies shared by the service.
type ServiceContext struct {
	Config   config.Config
	Redis    redisclient.RedisClient
	Registry *consul.ConsulRegistry
}

// NewServiceContext collects dependencies created by the application layer.
func NewServiceContext(
	cfg config.Config,
	redisClient redisclient.RedisClient,
	registry *consul.ConsulRegistry,

) *ServiceContext {
	return &ServiceContext{
		Config:   cfg,
		Redis:    redisClient,
		Registry: registry,
	}
}
