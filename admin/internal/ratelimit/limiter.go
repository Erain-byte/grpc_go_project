package ratelimit

import (
	"context"
	"time"
)

// 构造体用于存储限制结果，包括是否允许请求，剩余请求次数，重试时间
type Result struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
}

// 接口用于限制请求速率，返回是否允许请求，剩余请求次数，重试时间
type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (Result, error)
}
