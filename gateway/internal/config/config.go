package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Environment    string               `yaml:"environment" default:"development"`
	Name           string               `yaml:"name" default:"gateway-service"`
	Host           string               `yaml:"host" default:"localhost"`
	Port           int                  `yaml:"port" default:"8080"`
	GRPCPort       int                  `yaml:"grpc_port" default:"9080"`
	Database       DbConfig             `yaml:"database"`
	Redis          RedisConfig          `yaml:"redis"`
	Auth           AuthConfig           `yaml:"auth"`
	Consul         ConsulConfig         `yaml:"consul"`
	Logger         LoggerConfig         `yaml:"logger"`
	Service        ServiceConfig        `yaml:"service"`
	Routes         []RouteConfig        `yaml:"routes"`
	Shutdown       ShutdownConfig       `yaml:"shutdown"`
	Grpc           GrpcConfig           `yaml:"grpc"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Tracing        TracingConfig        `yaml:"tracing"`
	AntiReplay     AntiReplayConfig     `yaml:"anti_replay"`
	Cors           CORSConfig           `yaml:"cors"`
}

type DbConfig struct {
	Driver          string `yaml:"driver" default:"mysql"`
	Host            string `yaml:"host" default:"localhost"`
	Port            int    `yaml:"port" default:"3306"`
	Username        string `yaml:"username" default:"root"`
	Password        string `yaml:"password" default:"123456"`
	DBName          string `yaml:"dbname" default:"gateway_db"`
	MaxIdleConns    int    `yaml:"max_idle_conns" default:"10"`
	MaxOpenConns    int    `yaml:"max_open_conns" default:"100"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" default:"3600"`
}

// RedisConfig represents the configuration for Redis.
type RedisConfig struct {
	Host             string   `yaml:"host" default:"localhost"`
	Port             int      `yaml:"port" default:"6379"`
	ClusterAddresses []string `yaml:"cluster_addresses"`
	Password         string   `yaml:"password" default:""`
	DB               int      `yaml:"db" default:"0"`
	PoolSize         int      `yaml:"pool_size" default:"100"`
	MinIdleConns     int      `yaml:"min_idle_conns" default:"10"`
	MaxIdleConns     int      `yaml:"max_idle_conns" default:"50"`
	ConnMaxIdleTime  string   `yaml:"conn_max_idle_time" default:"30m"`
	ConnMaxLifetime  string   `yaml:"conn_max_lifetime" default:"1h"`
	DialTimeout      string   `yaml:"dial_timeout" default:"5s"`
	ReadTimeout      string   `yaml:"read_timeout" default:"3s"`
	WriteTimeout     string   `yaml:"write_timeout" default:"3s"`
	PoolTimeout      string   `yaml:"pool_timeout" default:"4s"`
	HealthRequired   bool     `yaml:"health_required" default:"false"`
}

// 是否开启集群模式
func (r *RedisConfig) IsClusterMode() bool {
	return len(r.ClusterAddresses) > 0
}

// 获取redis单节点
func (r *RedisConfig) GetRedisAddress() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// 判断当前环境是否为生产环境
func (c *Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(c.Environment))
	return env == "prod" || env == "production"
}

// JWT配置
type AuthConfig struct {
	AccessToken  AccessTokenConfig  `yaml:"access_token" mapstructure:"access_token"`
	RefreshToken RefreshTokenConfig `yaml:"refresh_token" mapstructure:"refresh_token"`
}

// AccessTokenConfig controls how the gateway validates short-lived JWTs.
// Secret is intentionally loaded from the environment rather than YAML.
type AccessTokenConfig struct {
	Issuers   []string `yaml:"issuers" mapstructure:"issuers"`
	Audience  string   `yaml:"audience" mapstructure:"audience"`
	Algorithm string   `yaml:"algorithm" mapstructure:"algorithm"`
	Expire    string   `yaml:"expire" mapstructure:"expire"`
	Secret    string   `yaml:"-" mapstructure:"-"`
}

// RefreshTokenConfig describes the refresh-token policy. Refresh-token values
// themselves must never be stored in configuration files.
type RefreshTokenConfig struct {
	Expire         string `yaml:"expire" mapstructure:"expire"`
	Rotate         bool   `yaml:"rotate" mapstructure:"rotate"`
	ReuseDetection bool   `yaml:"reuse_detection" mapstructure:"reuse_detection"`
	RedisKeyPrefix string `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"`
}

// conusl配置
type ConsulConfig struct {
	Address                 []string `yaml:"address"`
	Host                    string   `yaml:"host" default:"localhost"`
	Port                    int      `yaml:"port" default:"8500"`
	Token                   string   `yaml:"token" default:""`
	Scheme                  string   `yaml:"scheme" default:"http"`
	CheckInterval           string   `yaml:"check_interval" default:"10s"`
	CheckTimeout            string   `yaml:"check_timeout" default:"5s"`
	TTL                     string   `yaml:"ttl" default:"30s"`
	DeregisterCriticalAfter string   `yaml:"deregister_critical_after" default:"90s"`
	KeepAliveInterval       string   `yaml:"keepalive_interval" default:"10s"`
}

// GetAddresses 获取Consul地址列表，优先使用集群地址
func (c *ConsulConfig) GetAddresses() []string {
	if len(c.Address) > 0 {
		return c.Address
	}
	return []string{fmt.Sprintf("%s:%d", c.Host, c.Port)}
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `yaml:"level" default:"info"`
	Format     string `yaml:"format" default:"json"`
	Filename   string `yaml:"filename" default:"logs/gateway.log"`
	MaxSize    int    `yaml:"max_size" default:"100"`
	MaxBackups int    `yaml:"max_backups" default:"10"`
	MaxAge     int    `yaml:"max_age" default:"30"`
	Compress   bool   `yaml:"compress" default:"true"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Version     string     `yaml:"version"`
	CTags       []string   `yaml:"tags"`
	PublicAPIs  []string   `yaml:"public_apis"`
	AuthAPIs    []string   `yaml:"auth_apis"`
	CorsEnabled bool       `yaml:"cors_enabled"`
	CORS        CORSConfig `yaml:"cors"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Name      string `yaml:"name"`
	Path      string `yaml:"path"`
	Service   string `yaml:"service"`
	StripPath bool   `yaml:"strip_path"`
	Timeout   string `yaml:"timeout"`
}

// ShutdownConfig 关闭配置
type ShutdownConfig struct {
	Timeout string `yaml:"timeout" default:"5s"`
}

// GrpcConfig gRPC 客户端配置
type GrpcConfig struct {
	UseTLS             bool   `yaml:"use_tls" default:"false"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" default:"false"`
	CertFile           string `yaml:"cert_file" default:""`
	KeyFile            string `yaml:"key_file" default:""`
	CaFile             string `yaml:"ca_file" default:""`
	ServerName         string `yaml:"server_name" default:""`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled" default:"false"`
	RequestsPerSecond float64 `yaml:"requests_per_second" default:"100"`
	BurstSize         int     `yaml:"burst_size" default:"20"`
	ByIP              bool    `yaml:"by_ip" default:"true"`
	ByAPI             bool    `yaml:"by_api" default:"false"`
	FallbackToLocal   bool    `yaml:"fallback_to_local" default:"true"`
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled     bool   `yaml:"enabled" default:"false"`
	MaxFailures uint32 `yaml:"max_failures" default:"5"`
	Timeout     string `yaml:"timeout" default:"30s"`
	MinRequests uint32 `yaml:"min_requests" default:"10"`
	Interval    string `yaml:"interval" default:"60s"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled      bool    `yaml:"enabled" default:"false"`
	ServiceName  string  `yaml:"service_name" default:"gateway-service"`
	Endpoint     string  `yaml:"endpoint" default:"localhost:4317"`
	SamplerType  string  `yaml:"sampler_type" default:"const"`
	SamplerParam float64 `yaml:"sampler_param" default:"1.0"`
	UseTLS       bool    `yaml:"use_tls" default:"false"`
	CaFile       string  `yaml:"ca_file" default:""`
	CertFile     string  `yaml:"cert_file" default:""`
	KeyFile      string  `yaml:"key_file" default:""`
	ServerName   string  `yaml:"server_name" default:"otel-collector"`
}

// AntiReplayConfig 防重放配置
type AntiReplayConfig struct {
	Secret             string `yaml:"secret" default:""`
	Enabled            bool   `yaml:"enabled" default:"false"`
	TimestampTolerance int    `yaml:"timestamp_tolerance" default:"300"`
	NonceCacheSize     int    `yaml:"nonce_cache_size" default:"10000"`
	NonceExpireTime    int    `yaml:"nonce_expire_time" default:"600"`
	FallbackToLocal    bool   `yaml:"fallback_to_local" default:"true"`
}

//初始化配置

func InitConfig(configPath string) (*Config, error) {
	v := viper.New()
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("gateway")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./etc")
	}
	// 设置默认值
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	// 解析配置文件
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	applyEnvOverrides(&cfg)

	return &cfg, nil

}

// 处理环境变量覆盖
func applyEnvOverrides(cfg *Config) {

	if v := os.Getenv("GATEWAY_ENVIRONMENT"); v != "" {
		cfg.Environment = v
	}
	if v := os.Getenv("GATEWAY_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("GATEWAY_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("GATEWAY_ACCESS_TOKEN_SECRET"); v != "" {
		cfg.Auth.AccessToken.Secret = v
	} else if v := os.Getenv("GATEWAY_JWT_SECRET"); v != "" {

		cfg.Auth.AccessToken.Secret = v
	}
	if v := os.Getenv("GATEWAY_ANTI_REPLAY_SECRET"); v != "" {
		cfg.AntiReplay.Secret = v
	}
}
