package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/internal/logger"
	"gateway/internal/svc"
	"gateway/pkg/apperror"
)

// BaseForwarder 通用转发器基类（只负责转发逻辑）
type BaseForwarder[T any] struct {
	factory       clien.ClientFactory[T] // 客户端工厂函数
	svcCtx        *svc.ServiceContext
	serviceName   string               // 服务名称（用于日志）
	clientManager *clien.ClientManager // 客户端管理器
}

// NewBaseForwarder 创建通用转发器实例
func NewBaseForwarder[T any](
	svcCtx *svc.ServiceContext,
	factory clien.ClientFactory[T],
	clientManager *clien.ClientManager,
	serviceName string) *BaseForwarder[T] {
	return &BaseForwarder[T]{
		clientManager: clientManager,
		factory:       factory,
		svcCtx:        svcCtx,      // 服务上下文
		serviceName:   serviceName, // 服务名称
	}
}

// getClientFromManager 从 ClientManager 获取客户端
func (b *BaseForwarder[T]) GetClient(ctx context.Context) (T, error) {
	var zero T //return zero, nil

	if b.clientManager == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client manager is nil"))
	}
	if b.factory == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client factory is nil"))
	}
	if len(b.serviceName) == 0 {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("serviceName is empty"))
	}
	//return b.clientManager.GetClient(ctx, b.serviceName), nil
	clien, err := clien.CreateClient(ctx, b.clientManager, b.serviceName, b.factory)
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return clien, nil
}

// HealthCheck 检查服务健康状态
func (b *BaseForwarder[T]) HealthCheck(ctx context.Context) bool {
	if b.svcCtx == nil || b.svcCtx.Registry == nil {
		logger.SugaredLogger.Errorf("[%s] Registry not available", b.serviceName)
		return false
	}
	entries, err := b.svcCtx.Registry.DiscoverGRPCService(ctx, b.serviceName)
	if err != nil {
		logger.SugaredLogger.Errorf("[%s] DiscoverGRPCService error: %v", b.serviceName, err)
		return false
	}
	logger.SugaredLogger.Infof("[%s] DiscoverGRPCService result: %v", b.serviceName, entries)
	return len(entries) > 0
}
