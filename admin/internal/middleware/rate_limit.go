package middleware

import (
	"admin/internal/auth"
	"admin/internal/ratelimit"
	"admin/internal/runtimeconfig"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type RateLimitInterceptor struct {
	limiter ratelimit.Limiter
	logger  *zap.Logger
	store   *runtimeconfig.Store
	tracer  trace.Tracer
}

func NewRateLimitInterceptor(limiter ratelimit.Limiter, logger *zap.Logger, store *runtimeconfig.Store) (*RateLimitInterceptor, error) {
	if limiter == nil {
		return nil, fmt.Errorf("rate-limit limiter is nil")
	}
	if store == nil {
		return nil, fmt.Errorf("rate-limit store is nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("rate-limit logger is nil")
	}
	return &RateLimitInterceptor{
		limiter: limiter,
		logger:  logger,
		store:   store,
		tracer:  otel.Tracer("admin/internal/middleware/ratelimit"),
	}, nil
}

func (m *RateLimitInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {

		//Consul 会频繁调用健康检查接口，所以这里需要过滤掉健康检查接口
		if info.FullMethod == healthpb.Health_Check_FullMethodName {
			return handler(ctx, req)
		}
		//获取快照
		snapshot := m.store.Load()
		if snapshot == nil {
			return nil, status.Errorf(codes.Unavailable, "rate-limit snapshot is nil")
		}
		//当前一次prc 始终使用快照配置
		ratelimitCfg := snapshot.RetLimit
		if ratelimitCfg.Enabled == false {
			return handler(ctx, req)
		}

		key, rule, err := m.ruleAndKey(ctx, req, info.FullMethod, ratelimitCfg)
		if err != nil {
			return nil, err
		}
		result, err := m.check(ctx, key, rule, ratelimitCfg.RedisTimeout, info.FullMethod)
		if err != nil {
			m.logger.Error(
				"rate-limit Redis operation failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
			)
			return nil, status.Errorf(codes.Unavailable, "rate-limit Redis operation failed")
		}
		if !result.Allowed {
			m.logger.Warn(
				"gRPC request rejected by rate limiter",
				zap.String("method", info.FullMethod),
				zap.Duration("retry_after", result.RetryAfter),
				zap.Int64("limit", result.Limit),
			)

			return nil, status.Errorf(
				codes.ResourceExhausted,
				"too many requests; retry after %s",
				result.RetryAfter.Round(time.Second),
			)
		}
		return handler(ctx, req)
	}
}

// Stream 在建立流时执行一次滑动窗口检查。它限制建立流的频率，
// 不统计流内消息数量，也不限制流的存续时间。
func (m *RateLimitInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if info.FullMethod == healthpb.Health_Check_FullMethodName {
			return handler(srv, stream)
		}

		snapshot := m.store.Load()
		if snapshot == nil {
			return status.Error(codes.Unavailable, "rate-limit snapshot is nil")
		}
		ratelimitCfg := snapshot.RetLimit
		if !ratelimitCfg.Enabled {
			return handler(srv, stream)
		}

		ctx := stream.Context()
		key, rule, err := m.ruleAndKey(ctx, nil, info.FullMethod, ratelimitCfg)
		if err != nil {
			return err
		}
		result, err := m.check(ctx, key, rule, ratelimitCfg.RedisTimeout, info.FullMethod)
		if err != nil {
			m.logger.Error(
				"stream rate-limit Redis operation failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
			)
			return status.Error(codes.Unavailable, "rate-limit Redis operation failed")
		}
		if !result.Allowed {
			m.logger.Warn(
				"gRPC stream rejected by rate limiter",
				zap.String("method", info.FullMethod),
				zap.Duration("retry_after", result.RetryAfter),
				zap.Int64("limit", result.Limit),
			)
			return status.Errorf(
				codes.ResourceExhausted,
				"too many stream requests; retry after %s",
				result.RetryAfter.Round(time.Second),
			)
		}

		return handler(srv, stream)
	}
}

// check 只跟踪一次 Redis 限流判定，返回时统一结束 Span。
func (m *RateLimitInterceptor) check(
	ctx context.Context,
	key string,
	rule runtimeconfig.Rule,
	redisTimeout time.Duration,
	fullMethod string,
) (result ratelimit.Result, returnErr error) {
	spanCtx, span := m.tracer.Start(
		ctx,
		"rate_limit.check",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("rate_limit.backend", "redis"),
			attribute.Int64("rate_limit.limit", rule.Limit),
			attribute.String("rate_limit.window", rule.Window.String()),
			attribute.String("rpc.method", fullMethod),
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
			span.SetStatus(otelcodes.Error, "Redis rate-limit check failed")
		}
		span.End()
	}()

	redisCtx, cancel := context.WithTimeout(spanCtx, redisTimeout)
	defer cancel()

	return m.limiter.Allow(redisCtx, key, rule.Limit, rule.Window)
}

func (m *RateLimitInterceptor) ruleAndKey(ctx context.Context, req any, fullMethod string, ratelimitCfg runtimeconfig.RateLimitSnapshot) (string, runtimeconfig.Rule, error) {
	authInfo, ok := auth.FromContext(ctx)
	if !ok || strings.TrimSpace(authInfo.AdminID) == "" {
		return "", runtimeconfig.Rule{}, status.Error(
			codes.Unauthenticated,
			"authenticated admin identity is missing",
		)
	}
	return m.makeKey(ratelimitCfg.KeyPrefix, "admin", authInfo.AdminID, fullMethod), ratelimitCfg.Default, nil
}

func (m *RateLimitInterceptor) makeKey(keyPrefix string, dimension string, identity string, fullMethod string) string {
	//不把用户名、Refresh Token和AdminID明文写进Redis Key。
	sum := sha256.Sum256(
		[]byte(identity + "|" + fullMethod),
	)
	return fmt.Sprintf("%s:%s:%x", keyPrefix, dimension, sum[:16])
}
