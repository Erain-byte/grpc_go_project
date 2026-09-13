package ratelimit

import (
	redisclient "admin/internal/redis"
	"context"
	"testing"
	"time"
)

// 嵌入接口以隔离不涉及的 Redis 方法，仅检查 Allow 发给 Lua 的参数。
type evalRecorder struct {
	redisclient.RedisClient
	args     []interface{}
	response []interface{}
}

func (r *evalRecorder) Eval(_ context.Context, _ string, _ []string, args ...interface{}) (interface{}, error) {
	r.args = args
	return r.response, nil
}

func TestAllowUsesMilliseconds(t *testing.T) {
	recorder := &evalRecorder{response: []interface{}{int64(1), int64(1), int64(0)}}
	limiter, err := NewRedisSlidingWindow(recorder)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	result, err := limiter.Allow(context.Background(), "test:login", 10, time.Minute)
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatal(err)
	}
	now, ok := recorder.args[0].(int64)
	if !ok || now < before || now > after {
		t.Fatalf("current time must be Unix milliseconds between %d and %d, got %v", before, after, recorder.args[0])
	}
	if recorder.args[1] != int64(60000) {
		t.Fatalf("window must be 60000 milliseconds, got %v", recorder.args[1])
	}
	if !result.Allowed || result.Remaining != 9 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAllowConvertsRetryMilliseconds(t *testing.T) {
	recorder := &evalRecorder{response: []interface{}{int64(0), int64(10), int64(1500)}}
	limiter, err := NewRedisSlidingWindow(recorder)
	if err != nil {
		t.Fatal(err)
	}
	result, err := limiter.Allow(context.Background(), "test:login", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allowed || result.Remaining != 0 || result.RetryAfter != 1500*time.Millisecond {
		t.Fatalf("unexpected denied result: %+v", result)
	}
}
