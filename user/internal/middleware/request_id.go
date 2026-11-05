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
		if requestID == "" || len(requestID) > maxRequestIDLength {
			//生成一个request-id
			requestID = uuid.NewString()
		}
		ctx = context.WithValue(ctx, requsetIdCtxKey{}, requestID)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("request.id", requestID))
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDMetadataKey, requestID))
		return handler(ctx, req)
	}
}

// StreamRequestID 为流式 RPC 注入与一元 RPC 相同的 Request ID 和 Trace 属性。
func StreamRequestID() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		requestID := requestIDFromMetadata(stream.Context())
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = uuid.NewString()
		}
		ctx := context.WithValue(stream.Context(), requsetIdCtxKey{}, requestID)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("request.id", requestID))
		_ = stream.SetHeader(metadata.Pairs(RequestIDMetadataKey, requestID))
		return handler(srv, &requestIDServerStream{ServerStream: stream, ctx: ctx})
	}
}

type requestIDServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *requestIDServerStream) Context() context.Context { return s.ctx }

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
