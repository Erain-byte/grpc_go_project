package middleware

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDUsesIncomingMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(RequestIDMetadataKey, "request-123"),
	)

	got := invokeRequestID(t, ctx)
	if got != "request-123" {
		t.Fatalf("request ID = %q, want request-123", got)
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	got := invokeRequestID(t, context.Background())
	if got == "" {
		t.Fatal("request ID is empty")
	}
}

func invokeRequestID(t *testing.T, ctx context.Context) string {
	t.Helper()

	var got string
	_, err := RequestID()(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/admin.v1.AdminService/Test"},
		func(ctx context.Context, _ any) (any, error) {
			got = RequestIDFromContext(ctx)
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("RequestID() error = %v", err)
	}
	return got
}
