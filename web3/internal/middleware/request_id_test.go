package middleware

import (
	"context"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDPreservesIncomingValueAndAddsSpanAttribute(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	ctx, span := provider.Tracer("test").Start(context.Background(), "request")
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(RequestIDMetadataKey, "request-42"))

	var captured string
	_, err := RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, _ any) (any, error) {
		captured = RequestIDFromContext(ctx)
		return nil, nil
	})
	span.End()
	if err != nil {
		t.Fatalf("RequestID() error = %v", err)
	}
	if captured != "request-42" {
		t.Fatalf("request ID = %q", captured)
	}
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	found := false
	for _, attr := range ended[0].Attributes() {
		if string(attr.Key) == "request.id" && attr.Value.AsString() == "request-42" {
			found = true
		}
	}
	if !found {
		t.Fatal("request.id span attribute was not recorded")
	}
}

func TestRequestIDReplacesOversizedValue(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(RequestIDMetadataKey, strings.Repeat("a", maxRequestIDLength+1)))
	var captured string
	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, _ any) (any, error) {
		captured = RequestIDFromContext(ctx)
		return nil, nil
	})
	if captured == "" || len(captured) > maxRequestIDLength {
		t.Fatalf("generated request ID = %q", captured)
	}
}
