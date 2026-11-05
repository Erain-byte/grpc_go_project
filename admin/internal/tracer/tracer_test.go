package tracer

import (
	"context"
	"testing"

	"admin/internal/config"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestSamplerRespectsUnsampledRemoteParent(t *testing.T) {
	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}
	parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, Remote: true})
	parentContext := trace.ContextWithRemoteSpanContext(context.Background(), parent)

	result := createSampler(config.TracingConfig{SamplerType: "const", SamplerParam: 1}).ShouldSample(
		sdktrace.SamplingParameters{
			ParentContext: parentContext,
			TraceID:       traceID,
			Name:          "admin.request",
			Kind:          trace.SpanKindServer,
		},
	)
	if result.Decision != sdktrace.Drop {
		t.Fatalf("sampling decision = %v, want Drop", result.Decision)
	}
}

func TestNilManagerUsesNoopProvider(t *testing.T) {
	var manager *Manager
	if manager.Provider() == nil || manager.Tracer() == nil {
		t.Fatal("nil manager must return no-op tracing components")
	}
}
