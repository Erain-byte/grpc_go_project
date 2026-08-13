package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestIDPreservesCallerValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/test", func(c *gin.Context) {
		if got := c.GetString(RequestIDKey); got != "request-42" {
			t.Fatalf("request ID in context = %q, want request-42", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "request-42")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if got := recorder.Header().Get(RequestIDHeader); got != "request-42" {
		t.Fatalf("response request ID = %q, want request-42", got)
	}
}

func TestRequestIDGeneratesValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))
	if got := recorder.Header().Get(RequestIDHeader); got == "" {
		t.Fatal("response request ID is empty")
	}
}

func TestLoggerMiddlewareRecordsRequestAndTraceIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, observed := observer.New(zapcore.DebugLevel)
	engine := gin.New()
	engine.Use(RequestID())
	engine.Use(loggerMiddleware(zap.New(core), "gateway-service"))
	engine.GET("/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("TraceIDFromHex() error = %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("SpanIDFromHex() error = %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, "request-42")
	req = req.WithContext(trace.ContextWithSpanContext(req.Context(), spanContext))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["trace_id"] != traceID.String() {
		t.Fatalf("trace_id = %v, want %s", fields["trace_id"], traceID.String())
	}
	if fields["request_id"] != "request-42" {
		t.Fatalf("request_id = %v, want request-42", fields["request_id"])
	}
}

func TestLoggerMiddlewareDerivesApplicationErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, observed := observer.New(zapcore.DebugLevel)
	engine := gin.New()
	engine.Use(loggerMiddleware(zap.New(core), "gateway-service"))
	engine.GET("/test", func(c *gin.Context) {
		Fail(c, apperror.InvalidArgument("invalid request"))
	})

	engine.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/test", nil),
	)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	if entries[0].Level != zap.WarnLevel {
		t.Fatalf("log level = %s, want warn", entries[0].Level)
	}
	if got := entries[0].ContextMap()["status_code"]; got != int64(http.StatusBadRequest) {
		t.Fatalf("status_code = %v, want %d", got, http.StatusBadRequest)
	}
}
