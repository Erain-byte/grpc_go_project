package ratelimit

import (
	redisclient "admin/internal/redis"
	"admin/pkg/apperorr"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const slidingWindowScript = `
local key = KEYS[1]

local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local requestID = ARGV[4]

local windowStart = now - window

redis.call(
    "ZREMRANGEBYSCORE",
    key,
    "-inf",
    windowStart
)

local current = redis.call("ZCARD", key)

if current >= limit then
    local oldest = redis.call(
        "ZRANGE",
        key,
        0,
        0,
        "WITHSCORES"
    )

    local retryAfter = window

    if oldest[2] ~= nil then
        retryAfter = window - (now - tonumber(oldest[2]))

        if retryAfter < 0 then
            retryAfter = 0
        end
    end

    return {
        0,
        current,
        retryAfter
    }
end

redis.call(
    "ZADD",
    key,
    now,
    requestID
)

redis.call(
    "PEXPIRE",
    key,
    window
)

return {
    1,
    current + 1,
    0
}
`

type RedisSlidingWindow struct {
	// TODO
	redis redisclient.RedisClient
}

func NewRedisSlidingWindow(redisClient redisclient.RedisClient) (*RedisSlidingWindow, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	return &RedisSlidingWindow{
		redis: redisClient,
	}, nil
}

func (l *RedisSlidingWindow) Allow(ctx context.Context, key string, limit int64, window time.Duration) (Result, error) {
	if key == "" {
		return Result{}, apperorr.InvalidArgument(
			"rate-limit key is empty",
		)
	}
	if limit <= 0 {
		return Result{}, apperorr.InvalidArgument(
			"rate-limit limit is invalid",
		)
	}
	if window <= 0 {
		return Result{}, apperorr.InvalidArgument(
			"rate-limit window is invalid",
		)
	}
	value, err := l.redis.Eval(
		ctx,
		slidingWindowScript,
		[]string{key},
		time.Now().UnixNano(),
		window.Milliseconds(),
		limit,
		uuid.NewString(),
	)
	if err != nil {
		return Result{}, err
	}
	// TODO
	velues, ok := value.([]interface{})
	if !ok || len(velues) != 3 {
		return Result{}, fmt.Errorf(
			"unexpected Redis rate-limit result: %T",
			value,
		)
	}
	allowed, ok := velues[0].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"unexpected Redis rate-limit result: %T",
			velues[0],
		)
	}
	//获取第一个值
	current, ok := velues[1].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"unexpected Redis rate-limit result: %T",
			velues[1],
		)
	}
	retryMilliseconds, ok := velues[2].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"unexpected Redis rate-limit result: %T",
			velues[2],
		)
	}
	//当前窗口剩余的请求数量
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:    allowed == 1,
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMilliseconds) * time.Millisecond,
	}, nil
}
