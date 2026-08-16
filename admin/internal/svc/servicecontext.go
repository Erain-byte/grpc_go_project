package svc

import (
	"admin/internal/config"
	"admin/internal/consul"
	"admin/internal/database"
	"admin/internal/redis"

	"go.uber.org/zap"
)

// ServiceContext 是 Admin 服务的依赖容器。
//
// 这些对象由 app.Run 统一实例化并保存到这里，Handler、Service、Repository
// 可以按需引用同一份依赖，不需要在各自包中重复创建数据库或 Redis 客户端。
type ServiceContext struct {
	Config *config.Config
	Redis  redis.RedisClient
	DB     *database.GormClient
	Logger *zap.Logger
	Consul *consul.ConsulRegistry
}

func NewServiceContext(
	cfg *config.Config,
	redisClient redis.RedisClient,
	db *database.GormClient,
	log *zap.Logger,
	consulRegistry *consul.ConsulRegistry,
) *ServiceContext {
	// 这里没有创建新连接，只是把外部已经创建好的对象地址保存起来。
	return &ServiceContext{
		Config: cfg,
		Redis:  redisClient,
		DB:     db,
		Logger: log,
		Consul: consulRegistry,
	}
}
