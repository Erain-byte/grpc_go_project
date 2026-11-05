package middleware

import (
	"context"

	"web3/pkg/apperror"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorHandler 在 gRPC 边界统一转换错误；服务端故障会保留原始错误日志，响应不会泄露内部细节。
func ErrorHandler(logger *zap.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = zap.NewNop()
	}
	logger = logger.Named("error")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		response, err := handler(ctx, req)
		if err == nil {
			return response, nil
		}
		converted := apperror.ToGRPC(err)
		code := status.Code(converted)
		if code == codes.Internal || code == codes.Unavailable || code == codes.DataLoss || code == codes.DeadlineExceeded {
			fields := append(observabilityFields(ctx),
				zap.String("method", info.FullMethod),
				zap.String("grpc_code", code.String()),
				zap.Error(err),
			)
			logger.Error("gRPC handler failed", fields...)
		}
		return nil, converted
	}
}
