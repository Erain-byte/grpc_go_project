package forwarder

import (
	"context"
	"testing"

	"user/internal/middleware"

	"google.golang.org/grpc/metadata"
)

func TestOutgoingContextPropagatesAllowedMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		middleware.RequestIDMetadataKey, "request-42",
		"authorization", "Bearer token",
		"untrusted", "must-not-propagate",
	))
	outgoing := OutgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(outgoing)
	if !ok {
		t.Fatal("outgoing metadata is missing")
	}
	if got := md.Get(middleware.RequestIDMetadataKey); len(got) != 1 || got[0] != "request-42" {
		t.Fatalf("request ID = %#v", got)
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer token" {
		t.Fatalf("authorization = %#v", got)
	}
	if got := md.Get("untrusted"); len(got) != 0 {
		t.Fatalf("untrusted metadata was propagated: %#v", got)
	}
}
