package tracer

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	TracerProvider *sdktrace.TracerProvider
	Tracer         trace.Tracer
)
