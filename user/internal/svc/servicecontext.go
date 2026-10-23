package svc

import (
	"user/internal/config"
	"user/internal/consul"
	"user/internal/databases"
	grpcclient "user/internal/grpc"
	"user/internal/idgen"
	"user/internal/redis"
	"user/internal/sharding"

	"go.uber.org/zap"
)

type ServiceContext struct {
	Config     *config.Config
	Consul     *consul.ConsulRegistry
	Redis      redis.RedisClient
	Database   *databases.GormClient
	Router     *sharding.UserRouter
	IDs        idgen.Generator
	Logger     *zap.Logger
	GRPCclient *grpcclient.ClientManager
}

func NewServiceContext(
	cfg *config.Config,
	consulRegistry *consul.ConsulRegistry,
	redisClient redis.RedisClient,
	database *databases.GormClient,
	router *sharding.UserRouter,
	ids idgen.Generator,
	log *zap.Logger,
	grpcclient *grpcclient.ClientManager,
) *ServiceContext {
	return &ServiceContext{
		Config:     cfg,
		Consul:     consulRegistry,
		Redis:      redisClient,
		Database:   database,
		Router:     router,
		IDs:        ids,
		Logger:     log,
		GRPCclient: grpcclient,
	}
}
