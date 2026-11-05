package middleware

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	RequestIDMetadataKey = "x-request-id"
	maxRequestIDLength   = 128
)

type requestIDContextKey struct{}

// RequestID 复用上游 Request ID；缺失或长度异常时生成新的 UUID。
// ID 会写入 context、当前服务端 Span 属性和 gRPC 响应 metadata。
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := requestIDFromMetadata(ctx)
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = uuid.NewString()
		}
		ctx = context.WithValue(ctx, requestIDContextKey{}, requestID)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("request.id", requestID))
		// 真实 gRPC Server 会把它返回给调用方；直接调用拦截器的测试没有 transport，忽略该错误。
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDMetadataKey, requestID))
		return handler(ctx, req)
	}
}

func requestIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(RequestIDMetadataKey)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
