package svc

import (
	"user/internal/config"
	"user/internal/consul"
	"user/internal/redis"
)

type ServiceContext struct {
	Config *config.Config
	Conusl *consul.ConsulRegistry
	Redis  redis.RedisClient
}

func NewServiceContext(
	cfg *config.Config,
	consul *consul.ConsulRegistry,
	redis redis.RedisClient,
) *ServiceContext {
	return &ServiceContext{
		Config: cfg,
		Conusl: consul,
		Redis:  redis,
	}
}
