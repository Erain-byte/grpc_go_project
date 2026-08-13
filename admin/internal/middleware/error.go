package middleware

import (
	"admin/pkg/apperorr"
	"context"

	"google.golang.org/grpc"
)

// ErrorHandler converts errors returned by Admin handlers and services into
// stable gRPC status errors. It does not recover panics; recovery belongs to a
// separate interceptor.
func ErrorHandler() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return nil, apperorr.ToGRPC(err)
		}
		return resp, nil
	}
}
