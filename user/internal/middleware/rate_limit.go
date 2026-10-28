package middleware

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/time/rate"
)

// 本地令牌桶限流
type RateLimit struct {
	//开关
	enable bool
	//限制单个实列每秒的请求数量
	request *rate.Limiter
	//使用带缓冲的channel,作为并发信号
	unarySlots  chan struct{}
	streamSlots chan struct{}
}

// 构造函数
func NewRateLimit(enable bool, requestsPerSecond float64, burst int, maxUnary int, maxStream int) *RateLimit {
	if enable {
		return &RateLimit{
			enable:      enable,
			request:     rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
			unarySlots:  make(chan struct{}, maxUnary),
			streamSlots: make(chan struct{}, maxStream),
		}
	}
	return &RateLimit{}
}

// 排查健康检查
func isHealthMethod(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/")
}

// 拦截器
func (m *RateLimit) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if !m.enable || isHealthMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		//限制请求速率
		if !m.request.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		//限制并发量
		select {
		case m.unarySlots <- struct{}{}:
			defer func() { <-m.unarySlots }()
		case <-ctx.Done():
			return nil, status.FromContextError(
				ctx.Err(),
			).Err()
		default:
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")

		}
		return handler(ctx, req)
	}
}

// Stream 拦截器
func (m *RateLimit) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !m.enable || isHealthMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		//限制请求速率
		if !m.request.Allow() {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		//限制并发量
		select {
		case m.streamSlots <- struct{}{}:
			defer func() { <-m.streamSlots }()
		case <-ss.Context().Done():
			return status.FromContextError(ss.Context().Err()).Err()
		//快速失效不阻塞
		default:
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(srv, ss)
	}
}
