package middleware

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func observabilityFields(ctx context.Context) []zap.Field {
	fields := make([]zap.Field, 0, 3)
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanContext.TraceID().String()),
			zap.String("span_id", spanContext.SpanID().String()),
		)
	}
	return fields
}

func traceSpan(ctx context.Context) trace.Span { return trace.SpanFromContext(ctx) }
