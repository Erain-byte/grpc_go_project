package middleware

import (
	"admin/internal/auth"
	"admin/internal/config"
	"admin/internal/ratelimit"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	authv1 "github.com/Erain-byte/grpc_go_project/proto/auth/v1"
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
	//开关
	enabled          bool
	redisTimeout     time.Duration
	keyPrefix        string
	loginRule        parsedRateLimitRule
	refreshTokenRule parsedRateLimitRule
	defaultRule      parsedRateLimitRule
	tracer           trace.Tracer
}

type parsedRateLimitRule struct {
	limit  int64
	window time.Duration
}

func NewRateLimitInterceptor(cfg config.RateLimitConfig, limiter ratelimit.Limiter, logger *zap.Logger) (*RateLimitInterceptor, error) {
	redisTimeout, err := time.ParseDuration(cfg.RedisTimeout)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid rate-limit Redis timeout %q",
			cfg.RedisTimeout,
		)
	}
	loginRule, err := parseRateLimitRule("login", cfg.Login)
	if err != nil {
		return nil, err
	}
	refreshTokenRule, err := parseRateLimitRule("refresh token", cfg.RefreshToken)
	if err != nil {
		return nil, err
	}
	defaultRule, err := parseRateLimitRule("default", cfg.Default)
	if err != nil {
		return nil, err
	}
	if cfg.Enabled && limiter == nil {
		return nil, fmt.Errorf("rate-limit enabled but no limiter provided")
	}
	return &RateLimitInterceptor{
		limiter:          limiter,
		logger:           logger,
		enabled:          cfg.Enabled,
		redisTimeout:     redisTimeout,
		keyPrefix:        strings.TrimSpace(cfg.KeyPrefix),
		loginRule:        loginRule,
		refreshTokenRule: refreshTokenRule,
		defaultRule:      defaultRule,
		tracer:           otel.Tracer("admin/internal/middleware/ratelimit"),
	}, nil
}

func parseRateLimitRule(name string, cfg config.RateLimitRule) (parsedRateLimitRule, error) {
	if cfg.Limit <= 0 {
		return parsedRateLimitRule{}, fmt.Errorf(
			"invalid rate-limit %q: limit must be greater than 0",
			name,
		)
	}
	window, err := time.ParseDuration(cfg.Window)
	if err != nil || window <= 0 {
		return parsedRateLimitRule{}, fmt.Errorf(
			"invalid rate-limit %q: invalid window %q",
			name,
			cfg.Window,
		)
	}
	return parsedRateLimitRule{
		limit:  cfg.Limit,
		window: window,
	}, nil
}

func (m *RateLimitInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		if !m.enabled {
			return handler(ctx, req)
		}
		//Consul 会频繁调用健康检查接口，所以这里需要过滤掉健康检查接口
		if info.FullMethod == healthpb.Health_Check_FullMethodName {
			return handler(ctx, req)
		}
		key, rule, err := m.ruleAndKey(ctx, req, info.FullMethod)
		if err != nil {
			return nil, err
		}
		result, err := m.check(ctx, key, rule, info.FullMethod)
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

// check 只跟踪一次 Redis 限流判定，返回时统一结束 Span。
func (m *RateLimitInterceptor) check(
	ctx context.Context,
	key string,
	rule parsedRateLimitRule,
	fullMethod string,
) (result ratelimit.Result, returnErr error) {
	spanCtx, span := m.tracer.Start(
		ctx,
		"rate_limit.check",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("rate_limit.backend", "redis"),
			attribute.Int64("rate_limit.limit", rule.limit),
			attribute.String("rate_limit.window", rule.window.String()),
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

	redisCtx, cancel := context.WithTimeout(spanCtx, m.redisTimeout)
	defer cancel()

	return m.limiter.Allow(redisCtx, key, rule.limit, rule.window)
}

func (m *RateLimitInterceptor) ruleAndKey(ctx context.Context, req any, fullMethod string) (string, parsedRateLimitRule, error) {
	switch fullMethod {
	//登录接口需要限流
	case adminv1.AdminService_Login_FullMethodName:
		loginRequest, ok := req.(*adminv1.LoginRequest)
		if !ok {
			return "", parsedRateLimitRule{}, status.Error(codes.Internal, "invalid login request")
		}
		username := strings.ToLower(strings.TrimSpace(loginRequest.GetUsername()))
		if username == "" {
			return "", parsedRateLimitRule{}, status.Error(codes.InvalidArgument, "username is required")
		}
		return m.makeKey("login", username, fullMethod), m.loginRule, nil
	// 刷新令牌接口需要限流
	case adminv1.AdminService_RefreshToken_FullMethodName:
		refreshRequest, ok := req.(*adminv1.RefreshTokenRequest)
		if !ok {
			return "", parsedRateLimitRule{}, status.Error(codes.Internal, "invalid refresh token request")
		}
		refreshToken := strings.TrimSpace(refreshRequest.GetRefreshToken())
		if refreshToken == "" {
			return "", parsedRateLimitRule{}, status.Error(codes.InvalidArgument, "refresh token is required")
		}
		return m.makeKey("refresh token", refreshToken, fullMethod), m.refreshTokenRule, nil
	// 验证会话接口不需要限流
	case authv1.AuthService_ValidateSession_FullMethodName:
		sessionRequest, ok := req.(*authv1.ValidateSessionRequest)
		if !ok {
			return "", parsedRateLimitRule{}, status.Error(codes.Internal, "invalid session request")
		}
		identity := sessionRequest.GetSubjectType() + ":" + sessionRequest.GetSubjectId()
		return m.makeKey("session", identity, fullMethod), m.defaultRule, nil
	default:
		authInfo, ok := auth.FromContext(ctx)
		if !ok || strings.TrimSpace(authInfo.AdminID) == "" {
			return "", parsedRateLimitRule{}, status.Error(
				codes.Unauthenticated,
				"authenticated admin identity is missing",
			)
		}
		return m.makeKey("admin", authInfo.AdminID, fullMethod), m.defaultRule, nil
	}

}

func (m *RateLimitInterceptor) makeKey(dimension string, identity string, fullMethod string) string {
	//不把用户名、Refresh Token和AdminID明文写进Redis Key。
	sum := sha256.Sum256(
		[]byte(identity + "|" + fullMethod),
	)
	return fmt.Sprintf("%s:%s:%x", m.keyPrefix, dimension, sum[:16])
}
