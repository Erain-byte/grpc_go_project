package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"gateway/internal/config"
	"gateway/internal/ratelimit"
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

	enabled      bool
	limit        int64
	window       time.Duration
	redisTimeout time.Duration
	failClosed   bool
	tracer       trace.Tracer
}

func NewRateLimitMiddleware(cfg config.RateLimitConfig, distributed ratelimit.Limiter) (*RateLimitMiddleware, error) {

	if cfg.Limit <= 0 {
		return nil, fmt.Errorf(
			"rate-limit value must be positive",
		)
	}
	window, err := time.ParseDuration(cfg.Window)
	if err != nil || window <= 0 {
		return nil, fmt.Errorf(
			"invalid rate-limit window %q",
			cfg.Window,
		)
	}
	redisTimeout, err := time.ParseDuration(cfg.RedisTimeout)
	if err != nil || redisTimeout <= 0 {
		return nil, fmt.Errorf(
			"invalid rate-limit redis timeout %q",
			cfg.RedisTimeout,
		)
	}
	if cfg.Enabled && distributed == nil {
		return nil, fmt.Errorf(
			"rate-limit enabled but distributed limiter is not provided",
		)
	}
	return &RateLimitMiddleware{
		distributed:  distributed,
		enabled:      cfg.Enabled,
		limit:        cfg.Limit,
		window:       window,
		redisTimeout: redisTimeout,
		failClosed:   cfg.FailClosed,
		tracer:       otel.Tracer("gateway/internal/middleware/ratelimit"),
	}, nil
}

func (m *RateLimitMiddleware) Handle(c *gin.Context) {
	if !m.enabled {
		// TODO: implement
		c.Next()
		return
	}
	key := rateLimitKey(c)
	result, err := m.check(c.Request.Context(), key, c.FullPath())
	if err == nil {
		// Redis 正常时，按照分布式限流结果决定放行或拒绝。
		m.handleResult(c, result)
		return
	}

	// Redis 异常时不切换为单实例计数，避免多 Gateway 实例的限流额度失真。
	if m.failClosed {
		Fail(c, apperror.New(
			apperror.CodeUnavailable,
			"rate-limit service is unavailable",
			http.StatusServiceUnavailable,
		))
		return
	}
	// fail-open：限流组件失败时放行，并明确返回当前限流状态。
	c.Header(
		"X-RateLimit-Status",
		"unavailable",
	)
	c.Next()
}

// check 只跟踪一次 Redis 限流判定；方法返回时统一结束 Span，
// 不把后续业务 Handler 的执行时间计入限流耗时。
func (m *RateLimitMiddleware) check(
	ctx context.Context,
	key string,
	route string,
) (result ratelimit.Result, returnErr error) {
	spanCtx, span := m.tracer.Start(
		ctx,
		"rate_limit.check",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("rate_limit.backend", "redis"),
			attribute.Int64("rate_limit.limit", m.limit),
			attribute.String("rate_limit.window", m.window.String()),
			attribute.String("http.route", route),
		),
	)
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

	redisCtx, cancel := context.WithTimeout(spanCtx, m.redisTimeout)
	defer cancel()

	return m.distributed.Allow(redisCtx, key, m.limit, m.window)
}

func (m *RateLimitMiddleware) handleResult(c *gin.Context, result ratelimit.Result) {
	c.Header(
		"X-RateLimit-Limit",
		strconv.FormatInt(result.Limit, 10),
	)
	c.Header(
		"X-RateLimit-Remaining",
		strconv.FormatInt(result.Remaining, 10),
	)
	if result.Allowed {
		c.Next()
		return
	}

	retrySeconds := retryAfterSecods(result.RetryAfter)

	if retrySeconds < 1 {
		retrySeconds = 1
	}
	c.Header(
		"Retry-After",
		strconv.FormatInt(retrySeconds, 10),
	)
	Fail(c, apperror.New(
		apperror.CodeTooManyRequests,
		"too many requests",
		http.StatusTooManyRequests,
	))
}
func rateLimitKey(c *gin.Context) string {
	dimension := "ip"
	identity := strings.TrimSpace(c.ClientIP())
	if userId := strings.TrimSpace(c.GetString(ContextUserID)); userId != "" {
		dimension = "user"
		identity = userId
	}
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	sum := sha256.Sum256(
		[]byte(identity + "|" + route),
	)
	return fmt.Sprintf(
		"gateway : rate_limit:%s:%x",
		dimension,
		sum[:16],
	)
}

func retryAfterSecods(duration time.Duration) int64 {
	if duration <= 0 {
		return 1
	}
	seconds := int64((duration + time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}
