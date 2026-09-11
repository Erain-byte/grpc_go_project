package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"gateway/internal/ratelimit"
	"gateway/internal/runtimeconfig"
	"gateway/pkg/apperror"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type RateLimitMiddleware struct {
	distributed ratelimit.Limiter
	store       *runtimeconfig.Store
	tracer      trace.Tracer
}

func NewRateLimitMiddleware(distributed ratelimit.Limiter, store *runtimeconfig.Store) (*RateLimitMiddleware, error) {
	if distributed == nil {
		return nil, fmt.Errorf("distributed limiter is nil")
	}
	if store == nil {
		return nil, fmt.Errorf("rate-limit runtime config store is nil")
	}
	return &RateLimitMiddleware{distributed: distributed, store: store, tracer: otel.Tracer("gateway/internal/middleware/ratelimit")}, nil
}

func (m *RateLimitMiddleware) Handle(c *gin.Context) {
	snapshot := m.store.Load()
	if snapshot == nil {
		Fail(c, apperror.Unavailable("runtime config is unavailable"))
		return
	}
	// 本次请求始终使用同一份限流规则。
	rule := snapshot.RateLimit
	if !rule.Enabled {
		c.Next()
		return
	}
	result, err := m.check(c.Request.Context(), rateLimitKey(c), c.FullPath(), rule)
	if err == nil {
		m.handleResult(c, result)
		return
	}
	if rule.FailClosed {
		Fail(c, apperror.New(apperror.CodeUnavailable, "rate-limit service is unavailable", http.StatusServiceUnavailable))
		return
	}
	c.Header("X-RateLimit-Status", "unavailable")
	c.Next()
}

func (m *RateLimitMiddleware) check(ctx context.Context, key, route string, rule runtimeconfig.RateLimitRule) (result ratelimit.Result, returnErr error) {
	spanCtx, span := m.tracer.Start(ctx, "rate_limit.check", trace.WithSpanKind(trace.SpanKindInternal), trace.WithAttributes(
		attribute.String("rate_limit.backend", "redis"),
		attribute.Int64("rate_limit.limit", rule.Limit),
		attribute.String("rate_limit.window", rule.Window.String()),
		attribute.String("http.route", route),
	))
	defer func() {
		span.SetAttributes(
			attribute.Bool("rate_limit.allowed", returnErr == nil && result.Allowed),
			attribute.Int64("rate_limit.remaining", result.Remaining),
			attribute.Int64("rate_limit.retry_after_ms", result.RetryAfter.Milliseconds()),
		)
		if returnErr != nil {
			span.RecordError(returnErr)
			span.SetStatus(codes.Error, "Redis rate-limit check failed")
		}
		span.End()
	}()
	redisCtx, cancel := context.WithTimeout(spanCtx, rule.RedisTimeout)
	defer cancel()
	return m.distributed.Allow(redisCtx, key, rule.Limit, rule.Window)
}

func (m *RateLimitMiddleware) handleResult(c *gin.Context, result ratelimit.Result) {
	c.Header("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
	if result.Allowed {
		c.Next()
		return
	}
	c.Header("Retry-After", strconv.FormatInt(retryAfterSecods(result.RetryAfter), 10))
	Fail(c, apperror.New(apperror.CodeTooManyRequests, "too many requests", http.StatusTooManyRequests))
}

func rateLimitKey(c *gin.Context) string {
	dimension := "ip"
	identity := strings.TrimSpace(c.ClientIP())
	if userID := strings.TrimSpace(c.GetString(ContextUserID)); userID != "" {
		dimension, identity = "user", userID
	}
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	sum := sha256.Sum256([]byte(identity + "|" + route))
	return fmt.Sprintf("gateway:rate_limit:%s:%x", dimension, sum[:16])
}

func retryAfterSecods(duration time.Duration) int64 {
	if duration <= 0 {
		return 1
	}
	seconds := int64((duration + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
