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
	JWT            JwtConfig            `yaml:"jwt"`
	Consul         ConsulConfig         `yaml:"consul"`
	Logger         LoggerConfig         `yaml:"logger"`
	Service        ServiceConfig        `yaml:"service"`
	Routes         []RouteConfig        `yaml:"routes"`
	Shutdown       ShutdownConfig       `yaml:"shutdown"`
	Grpc           GrpcConfig           `yaml:"grpc"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`      // 限流配置
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"` // 熔断配置
	Tracing        TracingConfig        `yaml:"tracing"`         // 链路追踪配置
	AntiReplay     AntiReplayConfig     `yaml:"anti_replay"`     // 防重放配置
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
	Host             string   `yaml:"host" default:"localhost"` // 单节点地址（cluster_addresses为空时使用）
	Port             int      `yaml:"port" default:"6379"`      // 单节点端口（cluster_addresses为空时使用）
	ClusterAddresses []string `yaml:"cluster_addresses"`        // 集群地址列表，如 ["192.168.1.1:6379", "192.168.1.2:6379"]
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
type JwtConfig struct {
	Secret string `yaml:"secret"`
	Expire string `yaml:"expire" default:"24h"`
}

type AuthConfig struct {
	Token string `yaml:"token" default:""`
}

// conusl配置
type ConsulConfig struct {
	Address                 []string `yaml:"address"`                  // Consul地址列表，如 ["192.168.1.1:8500"]
	Host                    string   `yaml:"host" default:"localhost"` // 单节点地址（addresses为空时使用）
	Port                    int      `yaml:"port" default:"8500"`      // 单节点端口（addresses为空时使用）
	Token                   string   `yaml:"token" default:""`         // 访问凭证
	Scheme                  string   `yaml:"scheme" default:"http"`    // 访问协议
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
	Level      string `yaml:"level" default:"info"`                // 日志级别：debug, info, warn, error
	Format     string `yaml:"format" default:"json"`               // 日志格式：json, console
	Filename   string `yaml:"filename" default:"logs/gateway.log"` // 日志文件路径
	MaxSize    int    `yaml:"max_size" default:"100"`              // 单个日志文件最大大小（MB）
	MaxBackups int    `yaml:"max_backups" default:"10"`            // 保留的旧日志文件数量
	MaxAge     int    `yaml:"max_age" default:"30"`                // 日志文件保留天数
	Compress   bool   `yaml:"compress" default:"true"`             // 是否压缩旧日志文件
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
	UseTLS             bool   `yaml:"use_tls" default:"false"`              // 是否启用 TLS
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" default:"false"` // 跳过证书验证（仅用于开发/测试）
	CertFile           string `yaml:"cert_file" default:""`                 // 客户端证书文件路径
	KeyFile            string `yaml:"key_file" default:""`                  // 客户端私钥文件路径
	CaFile             string `yaml:"ca_file" default:""`                   // CA 证书文件路径
	ServerName         string `yaml:"server_name" default:""`               // TLS Server Name（可选）
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool    `yaml:"enabled" default:"false"`           // 是否启用限流
	RequestsPerSecond float64 `yaml:"requests_per_second" default:"100"` // 每秒请求数
	BurstSize         int     `yaml:"burst_size" default:"20"`           // 令牌桶容量（突发流量）
	ByIP              bool    `yaml:"by_ip" default:"true"`              // 是否按 IP 限流
	ByAPI             bool    `yaml:"by_api" default:"false"`            // 是否按 API 路径限流
	FallbackToLocal   bool    `yaml:"fallback_to_local" default:"true"`  // Redis unavailable fallback, disable in production.
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled     bool   `yaml:"enabled" default:"false"`   // 是否启用熔断
	MaxFailures uint32 `yaml:"max_failures" default:"5"`  // 最大失败次数
	Timeout     string `yaml:"timeout" default:"30s"`     // 熔断器打开后的恢复等待时间
	MinRequests uint32 `yaml:"min_requests" default:"10"` // 半开状态下的最小请求数
	Interval    string `yaml:"interval" default:"60s"`    // 统计窗口时间
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled      bool    `yaml:"enabled" default:"false"`                // 是否启用链路追踪
	ServiceName  string  `yaml:"service_name" default:"gateway-service"` // 服务名称
	Endpoint     string  `yaml:"endpoint" default:"localhost:4317"`      // OTLP gRPC Collector 地址（标准端口 4317）
	SamplerType  string  `yaml:"sampler_type" default:"const"`           // 采样器类型：const, probabilistic, ratelimiting
	SamplerParam float64 `yaml:"sampler_param" default:"1.0"`            // 采样参数（const: 0/1, probabilistic: 0.0-1.0）
	UseTLS       bool    `yaml:"use_tls" default:"false"`                // 是否启用 TLS（生产环境建议启用）
	CaFile       string  `yaml:"ca_file" default:""`                     // CA 证书文件路径（默认：/etc/certs/gateway/ca.crt）
	CertFile     string  `yaml:"cert_file" default:""`                   // 客户端证书文件路径（双向认证时使用）
	KeyFile      string  `yaml:"key_file" default:""`                    // 客户端私钥文件路径（双向认证时使用）
	ServerName   string  `yaml:"server_name" default:"otel-collector"`   // TLS Server Name（用于证书验证）
}

// AntiReplayConfig 防重放配置
type AntiReplayConfig struct {
	Secret             string `yaml:"secret" default:""`
	Enabled            bool   `yaml:"enabled" default:"false"`           // 是否启用防重放（开发环境建议关闭）
	TimestampTolerance int    `yaml:"timestamp_tolerance" default:"300"` // 时间戳容差（秒），默认5分钟
	NonceCacheSize     int    `yaml:"nonce_cache_size" default:"10000"`  // Nonce 缓存大小
	NonceExpireTime    int    `yaml:"nonce_expire_time" default:"600"`   // Nonce 过期时间（秒），默认10分钟
	FallbackToLocal    bool   `yaml:"fallback_to_local" default:"true"`  // Redis unavailable fallback, disable in production.
}

//初始化配置

func InitConfig(configPath string) (*Config, error) {
	v := viper.New() // 创建一个新的viper实例
	if configPath != "" {
		v.SetConfigFile(configPath) // 设置配置文件路径
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
	applyEnvOverrides(&cfg) // 处理环境变量覆盖

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
	if v := os.Getenv("GATEWAY_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("GATEWAY_AUTH_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	if v := os.Getenv("GATEWAY_ANTI_REPLAY_SECRET"); v != "" {
		cfg.AntiReplay.Secret = v
	}
}
