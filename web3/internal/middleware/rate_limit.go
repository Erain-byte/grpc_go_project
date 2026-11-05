package middleware

import (
	"context"
	"strings"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RateLimit protects one Web3 service instance with a token bucket and a
// hard concurrency ceiling. Gateway still owns the distributed/IP limit.
type RateLimit struct {
	enabled bool
	request *rate.Limiter
	slots   chan struct{}
}

func NewRateLimit(enabled bool, requestsPerSecond float64, burst, maxConcurrency int) *RateLimit {
	if !enabled {
		return &RateLimit{}
	}
	return &RateLimit{
		enabled: true,
		request: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
		slots:   make(chan struct{}, maxConcurrency),
	}
}

func (m *RateLimit) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !m.enabled || strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}
		if !m.request.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		select {
		case m.slots <- struct{}{}:
			defer func() { <-m.slots }()
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		default:
			return nil, status.Error(codes.ResourceExhausted, "concurrency limit exceeded")
		}
		return handler(ctx, req)
	}
}
