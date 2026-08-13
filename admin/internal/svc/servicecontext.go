package svc

import (
	"admin/internal/config"
	"admin/internal/consul"
	"admin/internal/database"
	"admin/internal/redis"

	"go.uber.org/zap"
)

type ServiceContext struct {
	Config config.Config
	Redis  redis.RedisClient
	DB     *database.GormClient
	Logger *zap.Logger
	Conusl consul.ConsulRegistry
}

func NewServiceContext(
	c config.Config,
	r redis.RedisClient,
	g *database.GormClient,
	log *zap.Logger,
	consul consul.ConsulRegistry,
) *ServiceContext {
	return &ServiceContext{
		Config: c,
		Redis:  r,
		DB:     g,
		Logger: log,
		Conusl: consul,
	}
}
