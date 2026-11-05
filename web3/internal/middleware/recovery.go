package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"web3/internal/tracer"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery 捕获业务处理链中的 panic，记录服务端堆栈，并只向调用方返回稳定的 Internal 错误。
func Recovery(logger *zap.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.Named("recovery")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, returnErr error) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			panicErr := fmt.Errorf("panic: %v", recovered)
			tracer.RecordError(traceSpan(ctx), panicErr, "gRPC handler panic")
			fields := append(observabilityFields(ctx),
				zap.String("method", info.FullMethod),
				zap.Any("panic", recovered),
				zap.ByteString("stacktrace", debug.Stack()),
			)
			logger.Error("gRPC panic recovered", fields...)
			response = nil
			returnErr = status.Error(codes.Internal, "internal server error")
		}()
		return handler(ctx, req)
	}
}
