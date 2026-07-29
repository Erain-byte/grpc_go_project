package logger

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestGetTraceID(t *testing.T) {
	want := "0102030405060708090a0b0c0d0e0f10"
	traceID, err := trace.TraceIDFromHex(want)
	if err != nil {
		t.Fatalf("parse trace ID: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatalf("parse span ID: %v", err)
	}

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	if got := GetTraceID(ctx); got != want {
		t.Fatalf("GetTraceID() = %q, want %q", got, want)
	}
}

func TestGetTraceIDWithoutValidSpanContext(t *testing.T) {
	if got := GetTraceID(context.Background()); got != "" {
		t.Fatalf("GetTraceID() = %q, want empty string", got)
	}
}
