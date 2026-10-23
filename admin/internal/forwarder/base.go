package forwarder

import (
	"context"
	"strings"

	grpcclient "admin/internal/grpcclient"
	apperror "admin/pkg/apperorr"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BaseForwarder 保存下游服务名称、gRPC 连接管理器和 protobuf 客户端工厂。
// ClientManager 负责服务发现与连接复用，BaseForwarder 负责创建具体的 protobuf 客户端。
type BaseForwarder[T any] struct {
	serviceName   string
	clientManager *grpcclient.ClientManager
	factory       func(grpc.ClientConnInterface) T
}

// OutgoingContext 从当前入站请求中读取 Bearer Token，并写入下游 gRPC 请求的 metadata。
// 下游服务可以据此独立验证最初由 Keycloak 签发的 Access Token。
func OutgoingContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", values[0])
}

func NewBaseForwarder[T any](clientManager *grpcclient.ClientManager, serviceName string, factory func(grpc.ClientConnInterface) T) *BaseForwarder[T] {
	return &BaseForwarder[T]{
		serviceName:   strings.TrimSpace(serviceName),
		clientManager: clientManager,
		factory:       factory,
	}
}

// Client 获取目标服务共享的 gRPC 连接，并创建对应的 protobuf 客户端。
// 服务发现和 grpc.ClientConn 的生命周期由 ClientManager 统一管理。
func (f *BaseForwarder[T]) Client(ctx context.Context) (T, error) {
	var zero T
	if f == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("forwarder is nil"))
	}
	if ctx == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	if f.clientManager == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client manager is nil"))
	}
	if f.serviceName == "" {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("service name is empty"))
	}
	if f.factory == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("client factory is nil"))
	}

	conn, err := f.clientManager.GetClient(ctx, f.serviceName)
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return f.factory(conn), nil
}
