package ratelimit

import (
	"context"
	"fmt"
	"time"

	redisclient "gateway/internal/redis"
	"gateway/pkg/apperror"

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
        retryAfter =
            window - (now - tonumber(oldest[2]))

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
	redis redisclient.RedisClient
}

func NewRedisSlidingWindow(
	redis redisclient.RedisClient,
) (*RedisSlidingWindow, error) {
	if redis == nil {
		return nil, fmt.Errorf("Redis client is nil")
	}

	return &RedisSlidingWindow{
		redis: redis,
	}, nil
}

func (l *RedisSlidingWindow) Allow(
	ctx context.Context,
	key string,
	limit int64,
	window time.Duration,
) (Result, error) {
	if key == "" {
		return Result{}, apperror.InvalidArgument(
			"rate-limit key is empty",
		)
	}

	if limit <= 0 {
		return Result{}, apperror.InvalidArgument(
			"rate-limit value must be positive",
		)
	}

	if window <= 0 {
		return Result{}, apperror.InvalidArgument(
			"rate-limit window must be positive",
		)
	}

	raw, err := l.redis.Eval(
		ctx,
		slidingWindowScript,
		[]string{key},
		time.Now().UnixMilli(),
		window.Milliseconds(),
		limit,
		uuid.NewString(),
	)
	if err != nil {
		return Result{}, err
	}

	values, ok := raw.([]any)
	if !ok || len(values) != 3 {
		return Result{}, fmt.Errorf(
			"unexpected Redis rate-limit result: %T",
			raw,
		)
	}

	allowedValue, ok := values[0].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"invalid allowed result: %T",
			values[0],
		)
	}

	current, ok := values[1].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"invalid request count: %T",
			values[1],
		)
	}

	retryMilliseconds, ok := values[2].(int64)
	if !ok {
		return Result{}, fmt.Errorf(
			"invalid retry-after value: %T",
			values[2],
		)
	}

	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	return Result{
		Allowed:   allowedValue == 1,
		Limit:     limit,
		Remaining: remaining,
		RetryAfter: time.Duration(retryMilliseconds) *
			time.Millisecond,
	}, nil
}
