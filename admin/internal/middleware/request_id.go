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

// RequestID propagates the caller's request ID or creates one when absent.
func RequestID() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		requestID := requestIDFromMetadata(ctx)
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = uuid.NewString()
		}

		ctx = context.WithValue(ctx, requestIDContextKey{}, requestID)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("request.id", requestID))
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDMetadataKey, requestID))
		return handler(ctx, req)
	}
}

// StreamRequestID 为流式 RPC 注入 Request ID，并把它关联到当前服务端 Span。
func StreamRequestID() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		requestID := requestIDFromMetadata(stream.Context())
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = uuid.NewString()
		}
		ctx := context.WithValue(stream.Context(), requestIDContextKey{}, requestID)
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

// RequestIDFromContext returns the request ID stored by RequestID.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
