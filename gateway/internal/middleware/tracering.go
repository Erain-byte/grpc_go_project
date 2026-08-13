package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing creates one server span for every inbound HTTP request.
func Tracing(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)

		spanName := c.Request.Method + " " + c.FullPath()
		if c.FullPath() == "" {
			spanName = c.Request.Method + " " + c.Request.URL.Path
		}
		ctx, span := tracer.Start(
			ctx,
			spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.path", c.Request.URL.Path),
			),
		)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)
		if spanContext := span.SpanContext(); spanContext.IsValid() {
			// Set this before a downstream handler writes the response body.
			c.Header("X-Trace-ID", spanContext.TraceID().String())
		}

		c.Next()

		statusCode := c.Writer.Status()
		if ginErr := c.Errors.Last(); ginErr != nil && statusCode < 400 {
			statusCode = FromGRPCError(ginErr.Err).HTTPStatus
		}
		span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}
		if ginErr := c.Errors.Last(); ginErr != nil {
			span.RecordError(ginErr.Err)
			span.SetStatus(codes.Error, ginErr.Error())
		}
	}
}
