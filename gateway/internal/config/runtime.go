package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RuntimeConfig 描述允许通过配置中心管理的 Gateway 运行策略。
// 密码、JWT Secret、Consul Token 和 TLS 私钥不得放入该结构。
type RuntimeConfig struct {
	Version        int                   `json:"version"`
	CORS           *CORSConfig           `json:"cors"`
	RateLimit      *RateLimitConfig      `json:"rate_limit"`
	CircuitBreaker *CircuitBreakerConfig `json:"circuit_breaker"`
	AntiReplay     *RuntimeAntiReplay    `json:"anti_replay"`
}

// RuntimeAntiReplay 不包含 HMAC Secret，Secret 继续由环境变量注入。
type RuntimeAntiReplay struct {
	Enabled                  bool `json:"enabled"`
	TimestampToleranceSecond int  `json:"timestamp_tolerance_seconds"`
	NonceExpireSecond        int  `json:"nonce_expire_seconds"`
	NonceCacheSize           int  `json:"nonce_cache_size"`
	FallbackToLocal          bool `json:"fallback_to_local"`
}

func ParseRuntimeConfig(data []byte) (*RuntimeConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg RuntimeConfig
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if cfg.Version <= 0 {
		return nil, fmt.Errorf("version must be positive")
	}
	if cfg.CORS == nil || cfg.RateLimit == nil || cfg.CircuitBreaker == nil || cfg.AntiReplay == nil {
		return nil, fmt.Errorf("cors, rate_limit, circuit_breaker and anti_replay are required")
	}
	if err := validateRuntimeCORS(*cfg.CORS); err != nil {
		return nil, fmt.Errorf("invalid cors: %w", err)
	}
	if err := validateRuntimeRateLimit(*cfg.RateLimit); err != nil {
		return nil, fmt.Errorf("invalid rate_limit: %w", err)
	}
	if err := validateRuntimeCircuitBreaker(*cfg.CircuitBreaker); err != nil {
		return nil, fmt.Errorf("invalid circuit_breaker: %w", err)
	}
	if cfg.AntiReplay.TimestampToleranceSecond <= 0 || cfg.AntiReplay.NonceExpireSecond <= 0 || cfg.AntiReplay.NonceCacheSize <= 0 {
		return nil, fmt.Errorf("invalid anti_replay: timeout and cache values must be positive")
	}
	return &cfg, nil
}

func validateRuntimeCORS(cfg CORSConfig) error {
	if len(cfg.AllowOrigins) == 0 || len(cfg.AllowMethods) == 0 {
		return fmt.Errorf("allow_origins and allow_methods must not be empty")
	}
	if cfg.MaxAge < 0 {
		return fmt.Errorf("max_age must not be negative")
	}
	for _, origin := range cfg.AllowOrigins {
		if strings.TrimSpace(origin) == "*" && cfg.AllowCredentials {
			return fmt.Errorf("wildcard origin cannot be combined with credentials")
		}
	}
	return nil
}

func validateRuntimeRateLimit(cfg RateLimitConfig) error {
	if cfg.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	if value, err := time.ParseDuration(cfg.Window); err != nil || value <= 0 {
		return fmt.Errorf("window %q is invalid", cfg.Window)
	}
	if value, err := time.ParseDuration(cfg.RedisTimeout); err != nil || value <= 0 {
		return fmt.Errorf("redis_timeout %q is invalid", cfg.RedisTimeout)
	}
	return nil
}

func validateRuntimeCircuitBreaker(cfg CircuitBreakerConfig) error {
	if cfg.MaxFailures == 0 || cfg.MinRequests == 0 {
		return fmt.Errorf("max_failures and min_requests must be positive")
	}
	for name, raw := range map[string]string{"timeout": cfg.Timeout, "interval": cfg.Interval} {
		if value, err := time.ParseDuration(raw); err != nil || value <= 0 {
			return fmt.Errorf("%s %q is invalid", name, raw)
		}
	}
	return nil
}
