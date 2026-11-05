package middleware

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorHandlerHidesUnknownErrorAndLogsOriginal(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	interceptor := ErrorHandler(zap.New(core))
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) {
		return nil, errors.New("database connection details")
	})
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "internal server error" {
		t.Fatalf("error = %v", err)
	}
	if observed.Len() != 1 {
		t.Fatalf("error log entries = %d, want 1", observed.Len())
	}
}
