package forwarder

import (
	"context"
	grpcclient "user/internal/grpc"
	"user/internal/middleware"
	"user/pkg/apperror"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

//本脚本实现了用户服务转发器，用于将用户服务请求转发到用户服务

// BaseForwarder 保存下游服务名称、gRPC 连接管理器和 protobuf 客户端工厂。
type BaseForwarder[T any] struct {
	//下游服务名称
	ServiceName string
	//gRPC 连接管理器
	connManager *grpcclient.ClientManager
	//protobuf 客户端工厂函数
	factory func(conn grpc.ClientConnInterface) T
}

// OutgoingContext 从当前入站请求中读取 Bearer Token，并写入下游 gRPC 请求的 metadata。
func OutgoingContext(ctx context.Context) context.Context {
	metadataPairs := make([]string, 0, 4)
	incomingRequestID := ""
	// 仅复制允许向下游传播的 metadata，避免透传全部调用方字段。
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if values := md.Get("authorization"); len(values) == 1 && values[0] != "" {
			metadataPairs = append(metadataPairs, "authorization", values[0])
		}
		if values := md.Get(middleware.RequestIDMetadataKey); len(values) > 0 && values[0] != "" {
			incomingRequestID = values[0]
			metadataPairs = append(metadataPairs, middleware.RequestIDMetadataKey, incomingRequestID)
		}
	}
	// 上游没有 ID 时，传播 User 拦截器生成的 Request ID。
	if incomingRequestID == "" {
		if requestID := middleware.RequestIDFromContext(ctx); requestID != "" {
			metadataPairs = append(metadataPairs, middleware.RequestIDMetadataKey, requestID)
		}
	}
	if len(metadataPairs) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, metadataPairs...)
}

// NewBaseForwarder 创建一个新的 BaseForwarder 实例。
func NewBaseForwarder[T any](serviceName string, connManager *grpcclient.ClientManager, factory func(conn grpc.ClientConnInterface) T) *BaseForwarder[T] {
	return &BaseForwarder[T]{
		ServiceName: serviceName,
		connManager: connManager,
		factory:     factory,
	}
}

// Client 获取目标服务共享的 gRPC 连接，并创建对应的 protobuf 客户端。
func (f *BaseForwarder[T]) Client(ctx context.Context) (T, error) {
	var zero T
	if f == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("forwarder is nil"))
	}
	if f.connManager == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("connManager is nil"))
	}
	if f.factory == nil {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("factory is nil"))
	}
	if f.ServiceName == "" {
		return zero, apperror.ToGRPC(apperror.InvalidArgument("serviceName is empty"))
	}
	conn, err := f.connManager.GetClient(ctx, f.ServiceName)
	if err != nil {
		return zero, apperror.ToGRPC(err)
	}
	return f.factory(conn), nil
}
