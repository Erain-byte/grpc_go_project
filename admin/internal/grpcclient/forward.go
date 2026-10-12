package grpcclient

import (
	apperror "admin/pkg/apperorr"
	"context"
	"strings"

	"google.golang.org/grpc"
)

// 泛型定义通用客户端工厂函数类型
type ForwardClientFactory[T any] func(conn grpc.ClientConnInterface) T

type ForwardClient[T any] struct {
	//客户端工厂函数
	factory ForwardClientFactory[T]
	//客户端管理器
	ClientManager *ClientManager
}

func NewForwardClient[T any](factory ForwardClientFactory[T], clientManager *ClientManager) *ForwardClient[T] {
	return &ForwardClient[T]{
		factory:       factory,
		ClientManager: clientManager,
	}
}

// 获取客服端泛型实列
func (c *ForwardClient[T]) ForwardBase(ctx context.Context, serviceName string) (T, error) {
	var zero T
	if ctx == nil {
		return zero, apperror.InvalidArgument("client ctx is nil")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return zero, apperror.InvalidArgument("client service name is empty")
	}
	if c.ClientManager == nil {
		return zero, apperror.InvalidArgument("client manager is nil")
	}
	conn, err := c.ClientManager.GetClient(ctx, serviceName)
	if err != nil {
		return zero, err
	}
	return c.factory(conn), nil
}
