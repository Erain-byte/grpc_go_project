// Package config defines and loads Admin service configuration.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config contains the long-lived configuration used by the Admin service.
type Config struct {
	Environment string           `yaml:"environment" mapstructure:"environment"`
	Name        string           `yaml:"name" mapstructure:"name"`
	Version     string           `yaml:"version" mapstructure:"version"`
	Host        string           `yaml:"host" mapstructure:"host"`
	GRPCPort    int              `yaml:"grpc_port" mapstructure:"grpc_port"`
	Database    DatabaseConfig   `yaml:"database" mapstructure:"database"`
	Redis       RedisConfig      `yaml:"redis" mapstructure:"redis"`
	RabbitMQ    RabbitMQConfig   `yaml:"rabbitmq" mapstructure:"rabbitmq"`
	Auth        AuthConfig       `yaml:"auth" mapstructure:"auth"`
	Consul      ConsulConfig     `yaml:"consul" mapstructure:"consul"`
	Logger      LoggerConfig     `yaml:"logger" mapstructure:"logger"`
	Shutdown    ShutdownConfig   `yaml:"shutdown" mapstructure:"shutdown"`
	GRPC        GRPCServerConfig `yaml:"grpc" mapstructure:"grpc"`
	Tracing     TracingConfig    `yaml:"tracing" mapstructure:"tracing"`
	RateLimit   RateLimitConfig  `yaml:"rate_limit" mapstructure:"rate_limit"`
}

// RabbitMQConfig 定义 Admin 服务的连接参数、发布确认、消费限速和操作日志拓扑。
// Password 只从环境变量加载，避免把明文密码提交到配置文件。
type RabbitMQConfig struct {
	Enabled               bool                 `yaml:"enabled" mapstructure:"enabled"`                                 // 是否启用 RabbitMQ。
	Host                  string               `yaml:"host" mapstructure:"host"`                                       // Broker 主机地址。
	Port                  int                  `yaml:"port" mapstructure:"port"`                                       // AMQP 端口，默认 5672。
	Username              string               `yaml:"username" mapstructure:"username"`                               // 登录用户名。
	Password              string               `yaml:"-" mapstructure:"-"`                                             // 仅从环境变量读取。
	VHost                 string               `yaml:"vhost" mapstructure:"vhost"`                                     // RabbitMQ 虚拟主机。
	ConnectionName        string               `yaml:"connection_name" mapstructure:"connection_name"`                 // 管理后台显示的连接名称。
	Heartbeat             string               `yaml:"heartbeat" mapstructure:"heartbeat"`                             // AMQP 心跳周期。
	DialTimeout           string               `yaml:"dial_timeout" mapstructure:"dial_timeout"`                       // 建立 TCP/AMQP 连接的超时。
	ReconnectInterval     string               `yaml:"reconnect_interval" mapstructure:"reconnect_interval"`           // 消费中断后的重连间隔。
	PublishConfirmTimeout string               `yaml:"publish_confirm_timeout" mapstructure:"publish_confirm_timeout"` // 等待 Broker Confirm 的最长时间。
	PrefetchCount         int                  `yaml:"prefetch_count" mapstructure:"prefetch_count"`                   // 单个消费者最多持有的未确认消息数。
	OperationLog          RabbitMQOperationLog `yaml:"operation_log" mapstructure:"operation_log"`                     // 操作日志消息拓扑。
}

// MqAddress 拼装不含账号密码的 Broker 网络地址，例如 127.0.0.1:5672。
func (m RabbitMQConfig) MqAddress() string {
	return net.JoinHostPort(
		strings.TrimSpace(m.Host),
		strconv.Itoa(m.Port),
	)
}

type RabbitMQOperationLog struct {
	// 主消息经过 Exchange + RoutingKey 路由到 Queue。
	Exchange        string `yaml:"exchange" mapstructure:"exchange"`
	ExchangeType    string `yaml:"exchange_type" mapstructure:"exchange_type"`
	RoutingKey      string `yaml:"routing_key" mapstructure:"routing_key"`
	Queue           string `yaml:"queue" mapstructure:"queue"`
	RetryExchange   string `yaml:"retry_exchange" mapstructure:"retry_exchange"`
	RetryRoutingKey string `yaml:"retry_routing_key" mapstructure:"retry_routing_key"`
	RetryQueue      string `yaml:"retry_queue" mapstructure:"retry_queue"`
	RetryDelay      string `yaml:"retry_delay" mapstructure:"retry_delay"`
	MaxRetries      int    `yaml:"max_retries" mapstructure:"max_retries"`
	// 超过重试次数或消息格式永久无效时进入死信队列，等待人工排查或补偿。
	DeadLetterExchange   string `yaml:"dead_letter_exchange" mapstructure:"dead_letter_exchange"`
	DeadLetterRoutingKey string `yaml:"dead_letter_routing_key" mapstructure:"dead_letter_routing_key"`
	DeadLetterQueue      string `yaml:"dead_letter_queue" mapstructure:"dead_letter_queue"`
}

type DatabaseConfig struct {
	Driver                string   `yaml:"driver" mapstructure:"driver"`
	Host                  string   `yaml:"host" mapstructure:"host"`
	Port                  int      `yaml:"port" mapstructure:"port"`
	Username              string   `yaml:"username" mapstructure:"username"`
	Password              string   `yaml:"-" mapstructure:"-"`
	DBName                string   `yaml:"dbname" mapstructure:"dbname"`
	SSLMode               string   `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	TimeZone              string   `yaml:"timezone" mapstructure:"timezone"`
	ConnectTimeout        string   `yaml:"connect_timeout" mapstructure:"connect_timeout"`
	ReadWriteSplitEnabled bool     `yaml:"read_write_split_enabled" mapstructure:"read_write_split_enabled"`
	ReplicaAddresses      []string `yaml:"replica_addresses" mapstructure:"replica_addresses"`
	MaxIdleConns          int      `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	MaxOpenConns          int      `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	ConnMaxIdleTime       string   `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
	ConnMaxLifetime       string   `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host             string   `yaml:"host" mapstructure:"host"`
	Port             int      `yaml:"port" mapstructure:"port"`
	ClusterAddresses []string `yaml:"cluster_addresses" mapstructure:"cluster_addresses"`
	Password         string   `yaml:"-" mapstructure:"-"`
	DB               int      `yaml:"db" mapstructure:"db"`
	PoolSize         int      `yaml:"pool_size" mapstructure:"pool_size"`
	MinIdleConns     int      `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
	MaxIdleConns     int      `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxIdleTime  string   `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
	ConnMaxLifetime  string   `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	DialTimeout      string   `yaml:"dial_timeout" mapstructure:"dial_timeout"`
	ReadTimeout      string   `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout     string   `yaml:"write_timeout" mapstructure:"write_timeout"`
	PoolTimeout      string   `yaml:"pool_timeout" mapstructure:"pool_timeout"`
	HealthRequired   bool     `yaml:"health_required" mapstructure:"health_required"`
}

func (c RedisConfig) IsClusterMode() bool {
	return len(c.ClusterAddresses) > 0
}

func (c RedisConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// GetRedisAddress
func (c RedisConfig) GetRedisAddress() []string {
	if len(c.ClusterAddresses) > 0 {
		return append([]string(nil), c.ClusterAddresses...)
	}
	return []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))}
}

type AuthConfig struct {
	AccessToken     AccessTokenConfig `yaml:"access_token" mapstructure:"access_token"`
	ProfileCacheTTL string            `yaml:"profile_cache_ttl" mapstructure:"profile_cache_ttl"`
}

type AccessTokenConfig struct {
	Issuer       string   `yaml:"issuer" mapstructure:"issuer"`
	Audience     string   `yaml:"audience" mapstructure:"audience"`
	Algorithm    string   `yaml:"algorithm" mapstructure:"algorithm"`
	JWKSURL      string   `yaml:"jwks_url" mapstructure:"jwks_url"`
	RolePriority []string `yaml:"role_priority" mapstructure:"role_priority"`
}

type ConsulConfig struct {
	Host                    string `yaml:"host" mapstructure:"host"`
	Port                    int    `yaml:"port" mapstructure:"port"`
	Token                   string `yaml:"-" mapstructure:"-"`
	Scheme                  string `yaml:"scheme" mapstructure:"scheme"`
	CheckHost               string `yaml:"check_host" mapstructure:"check_host"`
	CheckInterval           string `yaml:"check_interval" mapstructure:"check_interval"`
	CheckTimeout            string `yaml:"check_timeout" mapstructure:"check_timeout"`
	DeregisterCriticalAfter string `yaml:"deregister_critical_after" mapstructure:"deregister_critical_after"`
	RuntimeConfigKey        string `yaml:"runtime_config_key" mapstructure:"runtime_config_key"`
	RuntimeConfigRequired   bool   `yaml:"runtime_config_required" mapstructure:"runtime_config_required"`
}

func (c ConsulConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type LoggerConfig struct {
	Level      string `yaml:"level" mapstructure:"level"`
	Format     string `yaml:"format" mapstructure:"format"`
	Filename   string `yaml:"filename" mapstructure:"filename"`
	MaxSize    int    `yaml:"max_size" mapstructure:"max_size"`
	MaxBackups int    `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAge     int    `yaml:"max_age" mapstructure:"max_age"`
	Compress   bool   `yaml:"compress" mapstructure:"compress"`
}

type ShutdownConfig struct {
	Timeout string `yaml:"timeout" mapstructure:"timeout"`
}

// GRPCServerConfig controls the Admin inbound gRPC transport.
type GRPCServerConfig struct {
	// TLSServerName 是 Consul 健康检查验证的服务端证书 SAN，不是连接地址。
	TLSServerName string `yaml:"tls_server_name" mapstructure:"tls_server_name"`
	UseTLS        bool   `yaml:"use_tls" mapstructure:"use_tls"`
	CertFile      string `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile       string `yaml:"key_file" mapstructure:"key_file"`
	ClientCAFile  string `yaml:"client_ca_file" mapstructure:"client_ca_file"`
}

type TracingConfig struct {
	Enabled      bool    `yaml:"enabled" mapstructure:"enabled"`
	ServiceName  string  `yaml:"service_name" mapstructure:"service_name"`
	Endpoint     string  `yaml:"endpoint" mapstructure:"endpoint"`
	SamplerType  string  `yaml:"sampler_type" mapstructure:"sampler_type"`
	SamplerParam float64 `yaml:"sampler_param" mapstructure:"sampler_param"`
	UseTLS       bool    `yaml:"use_tls" mapstructure:"use_tls"`
	CAFile       string  `yaml:"ca_file" mapstructure:"ca_file"`
	CertFile     string  `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile      string  `yaml:"key_file" mapstructure:"key_file"`
	ServerName   string  `yaml:"server_name" mapstructure:"server_name"`
}

type Auth struct {
	AccessSecret string `yaml:"access_secret" mapstructure:"access_secret"`
	AccessExpire string `yaml:"access_secret_file" mapstructure:"access_secret_file"`
}

// 限流配置
type RateLimitConfig struct {
	Enabled      bool          `yaml:"enabled" mapstructure:"enabled" json:"enabled"`
	RedisTimeout string        `yaml:"redis_timeout" mapstructure:"redis_timeout" json:"redis_timeout"`
	KeyPrefix    string        `yaml:"key_prefix" mapstructure:"key_prefix" json:"key_prefix"`
	Default      RateLimitRule `yaml:"default" mapstructure:"default" json:"default"`
}

type RateLimitRule struct {
	Limit  int64  `yaml:"limit" mapstructure:"limit" json:"limit"`
	Window string `yaml:"window" mapstructure:"window" json:"window"`
}

func (c Config) IsProduction() bool {
	environment := strings.ToLower(strings.TrimSpace(c.Environment))
	return environment == "prod" || environment == "production"
}

// InitConfig loads YAML configuration and applies secret environment values.
func InitConfig(configPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	if strings.TrimSpace(configPath) != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("admin")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./etc")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read admin config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode admin config: %w", err)
	}
	applyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("environment", "development")
	v.SetDefault("name", "admin-service")
	v.SetDefault("version", "1.0.0")
	v.SetDefault("host", "127.0.0.1")
	v.SetDefault("grpc_port", 9081)
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.timezone", "Asia/Shanghai")
	v.SetDefault("database.connect_timeout", "5s")
	v.SetDefault("database.read_write_split_enabled", false)
	v.SetDefault("database.replica_addresses", []string{})
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("rabbitmq.enabled", false)
	v.SetDefault("rabbitmq.host", "127.0.0.1")
	v.SetDefault("rabbitmq.port", 5672)
	v.SetDefault("rabbitmq.vhost", "/grpc-go")
	v.SetDefault("rabbitmq.connection_name", "admin-service")
	v.SetDefault("rabbitmq.heartbeat", "30s")
	v.SetDefault("rabbitmq.dial_timeout", "5s")
	v.SetDefault("rabbitmq.reconnect_interval", "5s")
	v.SetDefault("rabbitmq.publish_confirm_timeout", "5s")
	v.SetDefault("rabbitmq.prefetch_count", 10)
	v.SetDefault("consul.host", "127.0.0.1")
	v.SetDefault("consul.port", 8500)
	v.SetDefault("consul.scheme", "http")
	v.SetDefault("shutdown.timeout", "10s")
	v.SetDefault("auth.profile_cache_ttl", "15m")
	//限流配置
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.redis_timeout", "300ms")
	v.SetDefault("rate_limit.key_prefix", "admin:rate_limit:")
	v.SetDefault("rate_limit.default.limit", 100)
	v.SetDefault("rate_limit.default.window", "1m")
}

func applyEnvironment(cfg *Config) {
	if value := os.Getenv("ADMIN_ENVIRONMENT"); value != "" {
		cfg.Environment = value
	}
	if value := os.Getenv("ADMIN_DB_PASSWORD"); value != "" {
		cfg.Database.Password = value
	}
	if value := os.Getenv("ADMIN_REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}
	if value := os.Getenv("ADMIN_RABBITMQ_PASSWORD"); value != "" {
		cfg.RabbitMQ.Password = value
	}
	if value := os.Getenv("ADMIN_CONSUL_TOKEN"); value != "" {
		cfg.Consul.Token = value
	}
}
func validateRateLimitConfig(cfg RateLimitConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		return fmt.Errorf("rate-limit key prefix is empty")
	}
	redisTimeout, err := time.ParseDuration(cfg.RedisTimeout)
	if err != nil || redisTimeout <= 0 {
		return fmt.Errorf(
			"rate-limit Redis timeout %q is invalid",
			cfg.RedisTimeout,
		)
	}
	rules := map[string]RateLimitRule{
		"default": cfg.Default,
	}
	for name, rule := range rules {
		if rule.Limit <= 0 {
			return fmt.Errorf(
				"rate-limit %s limit must be positive",
				name,
			)
		}
		window, err := time.ParseDuration(rule.Window)
		if err != nil || window <= 0 {
			return fmt.Errorf(
				"rate-limit %s window %q is invalid",
				name,
				rule.Window,
			)
		}
	}
	return nil
}

// Validate checks fields required before application dependencies are created.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("admin service name is empty")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("admin service host is empty")
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("admin gRPC port %d is outside 1..65535", c.GRPCPort)
	}
	if strings.ToLower(strings.TrimSpace(c.Database.Driver)) != "postgres" {
		return fmt.Errorf("admin database driver must be postgres")
	}
	if strings.TrimSpace(c.Database.Host) == "" {
		return fmt.Errorf("admin database host is empty")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("admin database port %d is outside 1..65535", c.Database.Port)
	}
	if strings.TrimSpace(c.Database.Username) == "" || strings.TrimSpace(c.Database.DBName) == "" {
		return fmt.Errorf("admin database username and database name are required")
	}
	validSSLModes := map[string]struct{}{
		"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {},
	}
	if _, ok := validSSLModes[strings.ToLower(strings.TrimSpace(c.Database.SSLMode))]; !ok {
		return fmt.Errorf("admin database SSL mode %q is invalid", c.Database.SSLMode)
	}
	if strings.TrimSpace(c.Database.TimeZone) == "" {
		return fmt.Errorf("admin database timezone is empty")
	}
	connectTimeout, err := time.ParseDuration(c.Database.ConnectTimeout)
	if err != nil || connectTimeout <= 0 {
		return fmt.Errorf("admin database connect timeout must be a positive duration")
	}
	if c.Database.MaxIdleConns < 0 || c.Database.MaxOpenConns <= 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("admin database connection pool limits are invalid")
	}
	for name, value := range map[string]string{
		"maximum idle time": c.Database.ConnMaxIdleTime,
		"maximum lifetime":  c.Database.ConnMaxLifetime,
	} {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return fmt.Errorf("admin database %s must be a positive duration", name)
		}
	}
	if c.Database.ReadWriteSplitEnabled {
		if len(c.Database.ReplicaAddresses) == 0 {
			return fmt.Errorf("database replica addresses are required when read/write splitting is enabled")
		}
		for _, address := range c.Database.ReplicaAddresses {
			if _, _, err := net.SplitHostPort(strings.TrimSpace(address)); err != nil {
				return fmt.Errorf("invalid database replica address %q: %w", address, err)
			}
		}
	}
	if c.RabbitMQ.Enabled {
		// 只有启用 RabbitMQ 时才要求连接参数和完整拓扑配置。
		if strings.TrimSpace(c.RabbitMQ.Host) == "" {
			return fmt.Errorf("RabbitMQ host is empty")
		}
		if c.RabbitMQ.Port < 1 || c.RabbitMQ.Port > 65535 {
			return fmt.Errorf("RabbitMQ port %d is outside 1..65535", c.RabbitMQ.Port)
		}
		if strings.TrimSpace(c.RabbitMQ.Username) == "" {
			return fmt.Errorf("RabbitMQ username is empty")
		}
		if strings.TrimSpace(c.RabbitMQ.VHost) == "" {
			return fmt.Errorf("RabbitMQ vhost is empty")
		}
		if c.RabbitMQ.PrefetchCount < 1 {
			return fmt.Errorf("RabbitMQ prefetch count must be positive")
		}
		// 统一校验所有以字符串表达的 Duration，避免运行阶段才发现格式错误。
		durations := map[string]string{
			"heartbeat":               c.RabbitMQ.Heartbeat,
			"dial timeout":            c.RabbitMQ.DialTimeout,
			"reconnect interval":      c.RabbitMQ.ReconnectInterval,
			"publish confirm timeout": c.RabbitMQ.PublishConfirmTimeout,
		}
		for name, value := range durations {
			duration, err := time.ParseDuration(value)
			if err != nil || duration <= 0 {
				return fmt.Errorf("RabbitMQ %s must be a positive duration", name)
			}
		}
		operationLog := c.RabbitMQ.OperationLog
		if strings.TrimSpace(operationLog.Exchange) == "" ||
			strings.TrimSpace(operationLog.ExchangeType) == "" ||
			strings.TrimSpace(operationLog.RoutingKey) == "" ||
			strings.TrimSpace(operationLog.Queue) == "" ||
			strings.TrimSpace(operationLog.RetryExchange) == "" ||
			strings.TrimSpace(operationLog.RetryRoutingKey) == "" ||
			strings.TrimSpace(operationLog.RetryQueue) == "" ||
			strings.TrimSpace(operationLog.DeadLetterExchange) == "" ||
			strings.TrimSpace(operationLog.DeadLetterRoutingKey) == "" ||
			strings.TrimSpace(operationLog.DeadLetterQueue) == "" {
			return fmt.Errorf("RabbitMQ operation-log topology is incomplete")
		}
		retryDelay, err := time.ParseDuration(operationLog.RetryDelay)
		if err != nil || retryDelay <= 0 {
			return fmt.Errorf("RabbitMQ operation-log retry delay must be a positive duration")
		}
		if operationLog.MaxRetries < 0 {
			return fmt.Errorf("RabbitMQ operation-log max retries cannot be negative")
		}
	}
	if strings.TrimSpace(c.Auth.AccessToken.Issuer) == "" {
		return fmt.Errorf("admin access-token issuer is empty")
	}
	if strings.TrimSpace(c.Auth.AccessToken.Audience) == "" {
		return fmt.Errorf("admin access-token audience is empty")
	}
	if strings.TrimSpace(c.Auth.AccessToken.Algorithm) != "RS256" {
		return fmt.Errorf("admin access-token algorithm must be RS256")
	}
	if strings.TrimSpace(c.Auth.AccessToken.JWKSURL) == "" {
		return fmt.Errorf("admin access-token JWKS URL is empty")
	}
	if len(c.Auth.AccessToken.RolePriority) == 0 {
		return fmt.Errorf("admin access-token role priority is empty")
	}
	profileCacheTTL, err := time.ParseDuration(c.Auth.ProfileCacheTTL)
	if err != nil || profileCacheTTL <= 0 {
		return fmt.Errorf("admin profile cache TTL must be a positive duration")
	}
	if c.GRPC.UseTLS && (strings.TrimSpace(c.GRPC.CertFile) == "" || strings.TrimSpace(c.GRPC.KeyFile) == "") {
		return fmt.Errorf("admin gRPC certificate and private key are required when TLS is enabled")
	}
	if err := validateRateLimitConfig(c.RateLimit); err != nil {
		return err
	}
	return nil
}
