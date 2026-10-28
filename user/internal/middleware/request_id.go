package middleware

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const RequestIDMetadataKey = "x-request-id"

// 占位符
type requsetIdCtxKey struct{}

// 拦截器
func RequestID() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		requestID := requestIDFromMetadata(ctx)
		if requestID == "" {
			//生成一个request-id
			requestID = uuid.NewString()
		}
		ctx = context.WithValue(ctx, requsetIdCtxKey{}, requestID)
		return handler(ctx, req)
	}
}

// 读取request-id
func requestIDFromMetadata(ctx context.Context) string {
	//读取metadata的信息
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	//从metadata中获取request-id
	value := md.Get(RequestIDMetadataKey)
	if len(value) == 0 {
		return ""
	}
	return strings.TrimSpace(value[0])
}

// 读取 RequestID
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requsetIdCtxKey{}).(string)
	return requestID
}
