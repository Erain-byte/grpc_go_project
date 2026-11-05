package middleware

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRecoveryConvertsPanicAndLogsCorrelationFields(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	recovery := Recovery(zap.New(core))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, "request-42"))
	info := &grpc.UnaryServerInfo{FullMethod: "/admin.v1.AdminService/Test"}

	response, err := RequestID()(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return recovery(ctx, req, info, func(context.Context, any) (any, error) {
			panic("boom")
		})
	})
	if response != nil || status.Code(err) != codes.Internal {
		t.Fatalf("response/code = %#v/%v, want nil/Internal", response, status.Code(err))
	}
	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "request-42" || fields["stacktrace"] == nil {
		t.Fatalf("panic log fields = %#v", fields)
	}
}
