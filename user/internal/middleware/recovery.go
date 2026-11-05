package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	tracing "user/internal/tracer"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery 捕获一元 RPC 的 panic，将错误写入当前 Span，并避免向调用方泄露堆栈。
func Recovery(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.recovery")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, returnErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				tracing.RecordError(traceSpan(ctx), panicErr, "gRPC handler panic")
				fields := append(observabilityFields(ctx),
					zap.String("method", info.FullMethod),
					zap.Any("panic", recovered),
					zap.ByteString("stacktrace", debug.Stack()),
				)
				log.Error("gRPC panic recovered", fields...)
				response = nil
				returnErr = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// StreamRecovery 为流式 RPC 提供与一元 RPC 相同的 panic 防御。
func StreamRecovery(log *zap.Logger) grpc.StreamServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.recovery")
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (returnErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				tracing.RecordError(traceSpan(stream.Context()), panicErr, "gRPC stream panic")
				fields := append(observabilityFields(stream.Context()),
					zap.String("method", info.FullMethod),
					zap.Any("panic", recovered),
					zap.ByteString("stacktrace", debug.Stack()),
				)
				log.Error("gRPC stream panic recovered", fields...)
				returnErr = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, stream)
	}
}
