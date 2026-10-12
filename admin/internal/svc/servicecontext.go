package svc

import (
	"admin/internal/config"
	"admin/internal/consul"
	"admin/internal/database"
	"admin/internal/grpcclient"
	"admin/internal/mq"
	"admin/internal/redis"
	"admin/internal/runtimeconfig"

	"go.uber.org/zap"
)

// ServiceContext 是 Admin 服务的依赖容器。
//
// 这些对象由 app.Run 统一实例化并保存到这里，Handler、Service、Repository
// 可以按需引用同一份依赖，不需要在各自包中重复创建数据库或 Redis 客户端。
type ServiceContext struct {
	Config      *config.Config
	Redis       redis.RedisClient
	DB          *database.GormClient
	Logger      *zap.Logger
	Consul      *consul.ConsulRegistry
	GRPCClients *grpcclient.ClientManager
	// OperationLogPublisher 由 app.Run 创建并注入，业务层不需要自行连接 RabbitMQ。
	OperationLogPublisher mq.OperationLogPublisher
	Runtime               *runtimeconfig.Store
}

func NewServiceContext(
	cfg *config.Config,
	redisClient redis.RedisClient,
	db *database.GormClient,
	log *zap.Logger,
	consulRegistry *consul.ConsulRegistry,
	grpcClients *grpcclient.ClientManager,
	operationLogPublisher mq.OperationLogPublisher,
	runtime *runtimeconfig.Store,
) *ServiceContext {
	// 这里没有创建新连接，只是把外部已经创建好的对象地址保存起来。
	return &ServiceContext{
		Config:                cfg,
		Redis:                 redisClient,
		DB:                    db,
		Logger:                log,
		Consul:                consulRegistry,
		GRPCClients:           grpcClients,
		OperationLogPublisher: operationLogPublisher,
		Runtime:               runtime,
	}
}

// Close releases resources whose lifecycle is owned by ServiceContext.
// Other long-lived dependencies can be added here as their ownership moves
// into ServiceContext.
func (s *ServiceContext) Close() {
	if s == nil {
		return
	}
	if s.GRPCClients != nil {
		s.GRPCClients.Close()
	}
}
