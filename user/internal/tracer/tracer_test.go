package tracer

import (
	"context"
	"testing"

	"user/internal/config"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestSamplerRespectsUnsampledRemoteParent(t *testing.T) {
	traceID := trace.TraceID{1}
	spanID := trace.SpanID{1}
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	parentContext := trace.ContextWithRemoteSpanContext(context.Background(), parent)

	sampler := createSampler(config.TracingConfig{SamplerType: "const", SamplerParam: 1})
	result := sampler.ShouldSample(sdktrace.SamplingParameters{
		ParentContext: parentContext,
		TraceID:       traceID,
		Name:          "user.request",
		Kind:          trace.SpanKindServer,
	})
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
