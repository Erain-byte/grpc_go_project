package middleware

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AccessLog 为每个一元 gRPC 请求记录一条结构化完成日志。
// 它只记录传输元数据，不记录请求体、Token 或钱包地址等敏感字段。
type AccessLog struct {
	logger *zap.Logger
}

func NewAccessLog(logger *zap.Logger) *AccessLog {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AccessLog{logger: logger.Named("grpc")}
}

func (m *AccessLog) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startedAt := time.Now()
		response, err := handler(ctx, req)
		code := status.Code(err)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("grpc_code", code.String()),
			zap.Duration("duration", time.Since(startedAt)),
		}
		fields = append(fields, observabilityFields(ctx)...)
		if err != nil {
			fields = append(fields, zap.Error(err))
		}
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			m.logger.Debug("gRPC health request completed", fields...)
			return response, err
		}

		switch {
		case code == codes.OK:
			m.logger.Info("gRPC request completed", fields...)
		case code == codes.InvalidArgument || code == codes.NotFound || code == codes.AlreadyExists ||
			code == codes.Unauthenticated || code == codes.PermissionDenied || code == codes.ResourceExhausted:
			m.logger.Warn("gRPC request completed", fields...)
		default:
			m.logger.Error("gRPC request completed", fields...)
		}
		return response, err
	}
}

func observabilityFields(ctx context.Context) []zap.Field {
	fields := make([]zap.Field, 0, 3)
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanContext.TraceID().String()),
			zap.String("span_id", spanContext.SpanID().String()),
		)
	}
	return fields
}

func traceSpan(ctx context.Context) trace.Span { return trace.SpanFromContext(ctx) }
