package middleware

import (
	"admin/internal/tracer"
	"admin/pkg/apperorr"
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorHandler converts errors returned by Admin handlers and services into
// stable gRPC status errors. It does not recover panics; recovery belongs to a
// separate interceptor.
func ErrorHandler(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.error")
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			converted := apperorr.ToGRPC(err)
			code := status.Code(converted)
			if isServerFailure(code) {
				tracer.RecordError(traceSpan(ctx), err, "gRPC handler failed")
				fields := append(observabilityFields(ctx),
					zap.String("method", info.FullMethod),
					zap.String("grpc_code", code.String()),
					zap.Error(err),
				)
				log.Error("gRPC handler failed", fields...)
			}
			return nil, converted
		}
		return resp, nil
	}
}

func StreamErrorHandler(log *zap.Logger) grpc.StreamServerInterceptor {
	if log == nil {
		log = zap.NewNop()
	}
	log = log.Named("grpc.error")
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, stream)
		if err == nil {
			return nil
		}
		converted := apperorr.ToGRPC(err)
		code := status.Code(converted)
		if isServerFailure(code) {
			tracer.RecordError(traceSpan(stream.Context()), err, "gRPC stream failed")
			fields := append(observabilityFields(stream.Context()),
				zap.String("method", info.FullMethod),
				zap.String("grpc_code", code.String()),
				zap.Error(err),
			)
			log.Error("gRPC stream failed", fields...)
		}
		return converted
	}
}

func isServerFailure(code codes.Code) bool {
	return code == codes.Internal || code == codes.Unavailable ||
		code == codes.DataLoss || code == codes.DeadlineExceeded
}
