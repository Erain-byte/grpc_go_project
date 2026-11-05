package middleware

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAccessLogUnaryRecordsRequestMetadata(t *testing.T) {
	t.Parallel()
	core, observed := observer.New(zapcore.DebugLevel)
	accessLog := NewAccessLog(zap.New(core)).Unary()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request-42"))
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	info := &grpc.UnaryServerInfo{FullMethod: "/web3.v1.Web3Service/GetWallets"}
	_, err := RequestID()(ctx, nil, info, func(ctx context.Context, req any) (any, error) {
		return accessLog(ctx, req, info, func(context.Context, any) (any, error) {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", status.Code(err))
	}

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Level != zap.WarnLevel {
		t.Fatalf("log level = %v, want warn", entry.Level)
	}
	fields := entry.ContextMap()
	if fields["method"] != "/web3.v1.Web3Service/GetWallets" {
		t.Fatalf("method = %v", fields["method"])
	}
	if fields["grpc_code"] != codes.InvalidArgument.String() {
		t.Fatalf("grpc_code = %v", fields["grpc_code"])
	}
	if fields["request_id"] != "request-42" {
		t.Fatalf("request_id = %v", fields["request_id"])
	}
	if fields["trace_id"] != traceID.String() || fields["span_id"] != spanID.String() {
		t.Fatalf("trace fields = %v/%v", fields["trace_id"], fields["span_id"])
	}
}
