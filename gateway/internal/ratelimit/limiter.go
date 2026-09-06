package ratelimit

import (
	"context"
	"time"
)

type Result struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, period time.Duration) (Result, error)
}
