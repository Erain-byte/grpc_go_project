package middleware

import (
	"admin/internal/tracer"
	"context"
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Recovery(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.recovery")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, returnErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				tracer.RecordError(traceSpan(ctx), panicErr, "gRPC handler panic")
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

func StreamRecovery(log *zap.Logger) grpc.StreamServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.recovery")
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (returnErr error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				panicErr := fmt.Errorf("panic: %v", recovered)
				tracer.RecordError(traceSpan(stream.Context()), panicErr, "gRPC stream panic")
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
