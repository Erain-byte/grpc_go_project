package forwarder

import (
	"context"
	"testing"

	"admin/internal/middleware"

	"google.golang.org/grpc/metadata"
)

func TestOutgoingContextPropagatesOnlyAllowedMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer token",
		middleware.RequestIDMetadataKey, "request-42",
		"x-untrusted", "secret",
	))
	outgoing := OutgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(outgoing)
	if !ok {
		t.Fatal("outgoing metadata is missing")
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer token" {
		t.Fatalf("authorization = %#v", got)
	}
	if got := md.Get(middleware.RequestIDMetadataKey); len(got) != 1 || got[0] != "request-42" {
		t.Fatalf("request ID = %#v", got)
	}
	if got := md.Get("x-untrusted"); len(got) != 0 {
		t.Fatalf("untrusted metadata was propagated: %#v", got)
	}
}
