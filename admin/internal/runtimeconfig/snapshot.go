package runtimeconfig

import (
	"admin/internal/config"
	"fmt"
	"strings"
	"time"
)

type Snapshot struct {
	Version  int
	RetLimit RateLimitSnapshot
}

type RateLimitSnapshot struct {
	Enabled      bool
	RedisTimeout time.Duration
	KeyPrefix    string
	Login        Rule
	RefreshToken Rule
	Default      Rule
}

type Rule struct {
	Limit  int64
	Window time.Duration
}

func BuildSnapshot(config *config.RuntimeConfig) (*Snapshot, error) {
	if config == nil || config.RateLimit == nil {
		return nil, fmt.Errorf("runtime CORS or rate-limit config is nil")

	}
	ratelimit := config.RateLimit
	redisTimot, err := time.ParseDuration(ratelimit.RedisTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid rate-limit redis timeout %q", ratelimit.RedisTimeout)
	}
	if redisTimot <= 0 {
		return nil, fmt.Errorf("invalid rate-limit redis timeout %q", ratelimit.RedisTimeout)
	}
	keyPrefix := strings.TrimSpace(ratelimit.KeyPrefix)
	if keyPrefix == "" {
		return nil, fmt.Errorf("invalid rate-limit key-prefix %q", ratelimit.KeyPrefix)
	}
	loginRule, err := buildRule("login", ratelimit.Login)
	if err != nil {
		return nil, err
	}
	refreshTokenRule, err := buildRule("refresh-token", ratelimit.RefreshToken)
	if err != nil {
		return nil, err
	}
	defaultRule, err := buildRule("default", ratelimit.Default)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Version: config.Version,
		RetLimit: RateLimitSnapshot{
			Enabled:      ratelimit.Enabled,
			RedisTimeout: redisTimot,
			KeyPrefix:    keyPrefix,
			Login:        loginRule,
			RefreshToken: refreshTokenRule,
			Default:      defaultRule,
		},
	}, nil
}

func buildRule(name string, rule config.RateLimitRule) (Rule, error) {
	if rule.Limit <= 0 {
		return Rule{}, fmt.Errorf("invalid rate-limit %s limit %d", name, rule.Limit)
	}
	window, err := time.ParseDuration(rule.Window)
	if err != nil || window <= 0 {
		return Rule{}, fmt.Errorf("invalid rate-limit %s window %q", name, rule.Window)
	}
	return Rule{
		Limit:  rule.Limit,
		Window: window,
	}, nil

}
