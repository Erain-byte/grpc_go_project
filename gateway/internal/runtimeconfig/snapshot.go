package runtimeconfig

import (
	"fmt"
	"gateway/internal/config"
	"time"
)

// Snapshot 是校验完成后发布给中间件的不可变运行配置。
type Snapshot struct {
	Version   int
	CORS      CORSRule
	RateLimit RateLimitRule
}

type CORSRule struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

type RateLimitRule struct {
	Enabled      bool
	Limit        int64
	Window       time.Duration
	RedisTimeout time.Duration
	FailClosed   bool
}

func BuildSnapshot(cfg *config.RuntimeConfig) (*Snapshot, error) {
	if cfg == nil || cfg.CORS == nil || cfg.RateLimit == nil {
		return nil, fmt.Errorf("runtime CORS or rate-limit config is nil")
	}
	window, err := time.ParseDuration(cfg.RateLimit.Window)
	if err != nil || window <= 0 {
		return nil, fmt.Errorf("invalid rate-limit window %q", cfg.RateLimit.Window)
	}
	redisTimeout, err := time.ParseDuration(cfg.RateLimit.RedisTimeout)
	if err != nil || redisTimeout <= 0 {
		return nil, fmt.Errorf("invalid rate-limit Redis timeout %q", cfg.RateLimit.RedisTimeout)
	}
	return &Snapshot{
		Version: cfg.Version,
		CORS: CORSRule{
			AllowOrigins:     append([]string(nil), cfg.CORS.AllowOrigins...),
			AllowMethods:     append([]string(nil), cfg.CORS.AllowMethods...),
			AllowHeaders:     append([]string(nil), cfg.CORS.AllowHeaders...),
			ExposeHeaders:    append([]string(nil), cfg.CORS.ExposeHeaders...),
			AllowCredentials: cfg.CORS.AllowCredentials,
			MaxAge:           cfg.CORS.MaxAge,
		},
		RateLimit: RateLimitRule{
			Enabled:      cfg.RateLimit.Enabled,
			Limit:        cfg.RateLimit.Limit,
			Window:       window,
			RedisTimeout: redisTimeout,
			FailClosed:   cfg.RateLimit.FailClosed,
		},
	}, nil
}
