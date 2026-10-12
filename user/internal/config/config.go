// Package config defines and loads USER service configuration.
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

// Config contains the long-lived configuration used by the USER service.
type Config struct {
	Environment   string          `yaml:"environment" mapstructure:"environment"`
	Name          string          `yaml:"name" mapstructure:"name"`
	Version       string          `yaml:"version" mapstructure:"version"`
	Host          string          `yaml:"host" mapstructure:"host"`
	GRPCPort      int             `yaml:"grpc_port" mapstructure:"grpc_port"`
	AdvertiseHost string          `yaml:"advertise_host" mapstructure:"advertise_host"`
	Database      DatabaseConfig  `yaml:"database" mapstructure:"database"`
	Redis         RedisConfig     `yaml:"redis" mapstructure:"redis"`
	RabbitMQ      RabbitMQConfig  `yaml:"rabbitmq" mapstructure:"rabbitmq"`
	GRPC          GRPCConfig      `yaml:"grpc" mapstructure:"grpc"`
	Consul        ConsulConfig    `yaml:"consul" mapstructure:"consul"`
	Logger        LoggerConfig    `yaml:"logger" mapstructure:"logger"`
	Tracing       TracingConfig   `yaml:"tracing" mapstructure:"tracing"`
	RateLimit     RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Shutdown      ShutdownConfig  `yaml:"shutdown" mapstructure:"shutdown"`
}

type DatabaseConfig struct {
	Enabled               bool     `yaml:"enabled" mapstructure:"enabled"`
	Driver                string   `yaml:"driver" mapstructure:"driver"`
	Host                  string   `yaml:"host" mapstructure:"host"`
	Port                  int      `yaml:"port" mapstructure:"port"`
	Username              string   `yaml:"username" mapstructure:"username"`
	Password              string   `yaml:"-" mapstructure:"-"`
	DBName                string   `yaml:"dbname" mapstructure:"dbname"`
	Charset               string   `yaml:"charset" mapstructure:"charset"`
	ParseTime             bool     `yaml:"parse_time" mapstructure:"parse_time"`
	Location              string   `yaml:"location" mapstructure:"location"`
	ReadWriteSplitEnabled bool     `yaml:"read_write_split_enabled" mapstructure:"read_write_split_enabled"`
	ReplicaAddresses      []string `yaml:"replica_addresses" mapstructure:"replica_addresses"`
	MaxIdleConns          int      `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	MaxOpenConns          int      `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	ConnMaxIdleTime       string   `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
	ConnMaxLifetime       string   `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
}

func (c DatabaseConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type RedisConfig struct {
	Enabled          bool     `yaml:"enabled" mapstructure:"enabled"`
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

func (c RedisConfig) GetAddresses() []string {
	if c.IsClusterMode() {
		return append([]string(nil), c.ClusterAddresses...)
	}
	return []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))}
}

type RabbitMQConfig struct {
	Enabled               bool         `yaml:"enabled" mapstructure:"enabled"`
	Host                  string       `yaml:"host" mapstructure:"host"`
	Port                  int          `yaml:"port" mapstructure:"port"`
	Username              string       `yaml:"username" mapstructure:"username"`
	Password              string       `yaml:"-" mapstructure:"-"`
	VHost                 string       `yaml:"vhost" mapstructure:"vhost"`
	ConnectionName        string       `yaml:"connection_name" mapstructure:"connection_name"`
	Heartbeat             string       `yaml:"heartbeat" mapstructure:"heartbeat"`
	DialTimeout           string       `yaml:"dial_timeout" mapstructure:"dial_timeout"`
	ReconnectInterval     string       `yaml:"reconnect_interval" mapstructure:"reconnect_interval"`
	PublishConfirmTimeout string       `yaml:"publish_confirm_timeout" mapstructure:"publish_confirm_timeout"`
	PrefetchCount         int          `yaml:"prefetch_count" mapstructure:"prefetch_count"`
	Task                  RabbitMQTask `yaml:"task" mapstructure:"task"`
}

func (c RabbitMQConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type RabbitMQTask struct {
	Exchange             string `yaml:"exchange" mapstructure:"exchange"`
	ExchangeType         string `yaml:"exchange_type" mapstructure:"exchange_type"`
	RoutingKey           string `yaml:"routing_key" mapstructure:"routing_key"`
	Queue                string `yaml:"queue" mapstructure:"queue"`
	RetryExchange        string `yaml:"retry_exchange" mapstructure:"retry_exchange"`
	RetryRoutingKey      string `yaml:"retry_routing_key" mapstructure:"retry_routing_key"`
	RetryQueue           string `yaml:"retry_queue" mapstructure:"retry_queue"`
	RetryDelay           string `yaml:"retry_delay" mapstructure:"retry_delay"`
	MaxRetries           int    `yaml:"max_retries" mapstructure:"max_retries"`
	DeadLetterExchange   string `yaml:"dead_letter_exchange" mapstructure:"dead_letter_exchange"`
	DeadLetterRoutingKey string `yaml:"dead_letter_routing_key" mapstructure:"dead_letter_routing_key"`
	DeadLetterQueue      string `yaml:"dead_letter_queue" mapstructure:"dead_letter_queue"`
}

type GRPCConfig struct {
	UseTLS             bool             `yaml:"use_tls" mapstructure:"use_tls"`
	TLSServerName      string           `yaml:"tls_server_name" mapstructure:"tls_server_name"`
	CertFile           string           `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile            string           `yaml:"key_file" mapstructure:"key_file"`
	ClientCAFile       string           `yaml:"client_ca_file" mapstructure:"client_ca_file"`
	Health             GRPCHealthConfig `yaml:"health" mapstructure:"health"`
	MaxRecvMessageSize int              `yaml:"max_recv_message_size" mapstructure:"max_recv_message_size"`
	MaxSendMessageSize int              `yaml:"max_send_message_size" mapstructure:"max_send_message_size"`
}

type GRPCHealthConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Service string `yaml:"service" mapstructure:"service"`
}

type ConsulConfig struct {
	Enabled                 bool                          `yaml:"enabled" mapstructure:"enabled"`
	Host                    string                        `yaml:"host" mapstructure:"host"`
	Port                    int                           `yaml:"port" mapstructure:"port"`
	Addresses               []string                      `yaml:"addresses" mapstructure:"addresses"`
	Token                   string                        `yaml:"-" mapstructure:"-"`
	Scheme                  string                        `yaml:"scheme" mapstructure:"scheme"`
	CheckHost               string                        `yaml:"check_host" mapstructure:"check_host"`
	CheckInterval           string                        `yaml:"check_interval" mapstructure:"check_interval"`
	CheckTimeout            string                        `yaml:"check_timeout" mapstructure:"check_timeout"`
	DeregisterCriticalAfter string                        `yaml:"deregister_critical_after" mapstructure:"deregister_critical_after"`
	RuntimeConfigKey        string                        `yaml:"runtime_config_key" mapstructure:"runtime_config_key"`
	RuntimeConfigRequired   bool                          `yaml:"runtime_config_required" mapstructure:"runtime_config_required"`
	RegistrationRequired    bool                          `yaml:"registration_required" mapstructure:"registration_required"`
	DeregisterOnShutdown    bool                          `yaml:"deregister_on_shutdown" mapstructure:"deregister_on_shutdown"`
	RegistrationMaintenance RegistrationMaintenanceConfig `yaml:"registration_maintenance" mapstructure:"registration_maintenance"`
}

func (c ConsulConfig) GetAddresses() []string {
	if len(c.Addresses) > 0 {
		return append([]string(nil), c.Addresses...)
	}
	return []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))}
}

type RegistrationMaintenanceConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	Interval   string `yaml:"interval" mapstructure:"interval"`
	MaxBackoff string `yaml:"max_backoff" mapstructure:"max_backoff"`
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

type RateLimitConfig struct {
	Enabled      bool          `yaml:"enabled" mapstructure:"enabled" json:"enabled"`
	RedisTimeout string        `yaml:"redis_timeout" mapstructure:"redis_timeout" json:"redis_timeout"`
	KeyPrefix    string        `yaml:"key_prefix" mapstructure:"key_prefix" json:"key_prefix"`
	Login        RateLimitRule `yaml:"login" mapstructure:"login" json:"login"`
	Register     RateLimitRule `yaml:"register" mapstructure:"register" json:"register"`
	Default      RateLimitRule `yaml:"default" mapstructure:"default" json:"default"`
}

type RateLimitRule struct {
	Limit  int64  `yaml:"limit" mapstructure:"limit" json:"limit"`
	Window string `yaml:"window" mapstructure:"window" json:"window"`
}

type ShutdownConfig struct {
	Timeout string `yaml:"timeout" mapstructure:"timeout"`
}

func (c Config) IsProduction() bool {
	environment := strings.ToLower(strings.TrimSpace(c.Environment))
	return environment == "prod" || environment == "production"
}

// InitConfig loads YAML configuration, applies secrets from environment
// variables, and validates the resulting configuration.
func InitConfig(configPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	if strings.TrimSpace(configPath) != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("user")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./etc")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read USER config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode USER config: %w", err)
	}
	applyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("environment", "development")
	v.SetDefault("name", "user-service")
	v.SetDefault("version", "1.0.0")
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("grpc_port", 9083)
	v.SetDefault("advertise_host", "127.0.0.1")
	v.SetDefault("database.enabled", false)
	v.SetDefault("database.driver", "mysql")
	v.SetDefault("database.charset", "utf8mb4")
	v.SetDefault("database.parse_time", true)
	v.SetDefault("database.location", "Local")
	v.SetDefault("database.replica_addresses", []string{})
	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.cluster_addresses", []string{})
	v.SetDefault("rabbitmq.enabled", false)
	v.SetDefault("rabbitmq.host", "127.0.0.1")
	v.SetDefault("rabbitmq.port", 5672)
	v.SetDefault("rabbitmq.vhost", "/grpc-go")
	v.SetDefault("rabbitmq.connection_name", "user-service")
	v.SetDefault("consul.enabled", true)
	v.SetDefault("consul.host", "127.0.0.1")
	v.SetDefault("consul.port", 8500)
	v.SetDefault("consul.addresses", []string{})
	v.SetDefault("consul.scheme", "http")
	v.SetDefault("shutdown.timeout", "10s")
}

func applyEnvironment(cfg *Config) {
	if value := os.Getenv("USER_ENVIRONMENT"); value != "" {
		cfg.Environment = value
	}
	if value := os.Getenv("USER_DB_PASSWORD"); value != "" {
		cfg.Database.Password = value
	}
	if value := os.Getenv("USER_REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}
	if value := os.Getenv("USER_RABBITMQ_PASSWORD"); value != "" {
		cfg.RabbitMQ.Password = value
	}
	if value := os.Getenv("USER_CONSUL_TOKEN"); value != "" {
		cfg.Consul.Token = value
	}
}

func validatePositiveDuration(name, value string) error {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
	}
	return nil
}

func validateHostPort(name, address string) error {
	if _, _, err := net.SplitHostPort(strings.TrimSpace(address)); err != nil {
		return fmt.Errorf("invalid %s address %q: %w", name, address, err)
	}
	return nil
}

func validateRateLimit(cfg RateLimitConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		return fmt.Errorf("rate-limit key prefix is empty")
	}
	if err := validatePositiveDuration("rate-limit Redis timeout", cfg.RedisTimeout); err != nil {
		return err
	}
	rules := map[string]RateLimitRule{
		"login":    cfg.Login,
		"register": cfg.Register,
		"default":  cfg.Default,
	}
	for name, rule := range rules {
		if rule.Limit <= 0 {
			return fmt.Errorf("rate-limit %s limit must be positive", name)
		}
		if err := validatePositiveDuration("rate-limit "+name+" window", rule.Window); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks all fields required before application dependencies are built.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("USER service name is empty")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("USER service host is empty")
	}
	if c.GRPCPort < 1 || c.GRPCPort > 65535 {
		return fmt.Errorf("USER gRPC port %d is outside 1..65535", c.GRPCPort)
	}
	if strings.TrimSpace(c.AdvertiseHost) == "" || c.AdvertiseHost == "0.0.0.0" {
		return fmt.Errorf("USER advertise host must be a reachable address")
	}

	if c.Database.Enabled {
		if strings.TrimSpace(c.Database.Driver) == "" || strings.TrimSpace(c.Database.Host) == "" ||
			strings.TrimSpace(c.Database.Username) == "" || strings.TrimSpace(c.Database.DBName) == "" {
			return fmt.Errorf("database connection configuration is incomplete")
		}
		if c.Database.Port < 1 || c.Database.Port > 65535 {
			return fmt.Errorf("database port %d is outside 1..65535", c.Database.Port)
		}
		if c.Database.MaxIdleConns < 0 || c.Database.MaxOpenConns <= 0 ||
			c.Database.MaxIdleConns > c.Database.MaxOpenConns {
			return fmt.Errorf("database connection pool configuration is invalid")
		}
		if err := validatePositiveDuration("database connection max idle time", c.Database.ConnMaxIdleTime); err != nil {
			return err
		}
		if err := validatePositiveDuration("database connection max lifetime", c.Database.ConnMaxLifetime); err != nil {
			return err
		}
		if c.Database.ReadWriteSplitEnabled && len(c.Database.ReplicaAddresses) == 0 {
			return fmt.Errorf("database replica addresses are required when read/write splitting is enabled")
		}
		for _, address := range c.Database.ReplicaAddresses {
			if err := validateHostPort("database replica", address); err != nil {
				return err
			}
		}
	}

	if c.Redis.Enabled {
		for _, address := range c.Redis.GetAddresses() {
			if err := validateHostPort("Redis", address); err != nil {
				return err
			}
		}
		if c.Redis.DB < 0 || c.Redis.PoolSize <= 0 || c.Redis.MinIdleConns < 0 ||
			c.Redis.MaxIdleConns < 0 || c.Redis.MaxIdleConns > c.Redis.PoolSize {
			return fmt.Errorf("Redis pool configuration is invalid")
		}
		for name, value := range map[string]string{
			"Redis connection max idle time": c.Redis.ConnMaxIdleTime,
			"Redis connection max lifetime":  c.Redis.ConnMaxLifetime,
			"Redis dial timeout":             c.Redis.DialTimeout,
			"Redis read timeout":             c.Redis.ReadTimeout,
			"Redis write timeout":            c.Redis.WriteTimeout,
			"Redis pool timeout":             c.Redis.PoolTimeout,
		} {
			if err := validatePositiveDuration(name, value); err != nil {
				return err
			}
		}
	}

	if c.RabbitMQ.Enabled {
		if strings.TrimSpace(c.RabbitMQ.Host) == "" || strings.TrimSpace(c.RabbitMQ.Username) == "" ||
			strings.TrimSpace(c.RabbitMQ.VHost) == "" {
			return fmt.Errorf("RabbitMQ connection configuration is incomplete")
		}
		if c.RabbitMQ.Port < 1 || c.RabbitMQ.Port > 65535 || c.RabbitMQ.PrefetchCount <= 0 {
			return fmt.Errorf("RabbitMQ port or prefetch count is invalid")
		}
		for name, value := range map[string]string{
			"RabbitMQ heartbeat":               c.RabbitMQ.Heartbeat,
			"RabbitMQ dial timeout":            c.RabbitMQ.DialTimeout,
			"RabbitMQ reconnect interval":      c.RabbitMQ.ReconnectInterval,
			"RabbitMQ publish confirm timeout": c.RabbitMQ.PublishConfirmTimeout,
			"RabbitMQ task retry delay":        c.RabbitMQ.Task.RetryDelay,
		} {
			if err := validatePositiveDuration(name, value); err != nil {
				return err
			}
		}
		task := c.RabbitMQ.Task
		if strings.TrimSpace(task.Exchange) == "" || strings.TrimSpace(task.ExchangeType) == "" ||
			strings.TrimSpace(task.RoutingKey) == "" || strings.TrimSpace(task.Queue) == "" ||
			strings.TrimSpace(task.RetryExchange) == "" || strings.TrimSpace(task.RetryRoutingKey) == "" ||
			strings.TrimSpace(task.RetryQueue) == "" || strings.TrimSpace(task.DeadLetterExchange) == "" ||
			strings.TrimSpace(task.DeadLetterRoutingKey) == "" || strings.TrimSpace(task.DeadLetterQueue) == "" {
			return fmt.Errorf("RabbitMQ task topology is incomplete")
		}
		if task.MaxRetries < 0 {
			return fmt.Errorf("RabbitMQ task max retries cannot be negative")
		}
	}

	if c.GRPC.UseTLS && (strings.TrimSpace(c.GRPC.CertFile) == "" ||
		strings.TrimSpace(c.GRPC.KeyFile) == "" || strings.TrimSpace(c.GRPC.ClientCAFile) == "") {
		return fmt.Errorf("gRPC certificate, private key, and client CA are required when TLS is enabled")
	}
	if c.GRPC.UseTLS && strings.TrimSpace(c.GRPC.TLSServerName) == "" {
		return fmt.Errorf("gRPC TLS server name is required when TLS is enabled")
	}
	if c.GRPC.MaxRecvMessageSize <= 0 || c.GRPC.MaxSendMessageSize <= 0 {
		return fmt.Errorf("gRPC message-size limits must be positive")
	}

	if c.Consul.Enabled {
		for _, address := range c.Consul.GetAddresses() {
			if err := validateHostPort("Consul", address); err != nil {
				return err
			}
		}
		if c.Consul.Scheme != "http" && c.Consul.Scheme != "https" {
			return fmt.Errorf("Consul scheme must be http or https")
		}
		if strings.TrimSpace(c.Consul.CheckHost) == "" {
			return fmt.Errorf("Consul check host is empty")
		}
		for name, value := range map[string]string{
			"Consul check interval":            c.Consul.CheckInterval,
			"Consul check timeout":             c.Consul.CheckTimeout,
			"Consul deregister critical after": c.Consul.DeregisterCriticalAfter,
		} {
			if err := validatePositiveDuration(name, value); err != nil {
				return err
			}
		}
		if c.Consul.RuntimeConfigRequired && strings.TrimSpace(c.Consul.RuntimeConfigKey) == "" {
			return fmt.Errorf("Consul runtime config key is required")
		}
		if c.Consul.RegistrationMaintenance.Enabled {
			if err := validatePositiveDuration("registration maintenance interval", c.Consul.RegistrationMaintenance.Interval); err != nil {
				return err
			}
			if err := validatePositiveDuration("registration maintenance max backoff", c.Consul.RegistrationMaintenance.MaxBackoff); err != nil {
				return err
			}
		}
	} else if c.Consul.RuntimeConfigRequired || c.Consul.RegistrationRequired {
		return fmt.Errorf("Consul must be enabled when registration or runtime config is required")
	}

	if strings.TrimSpace(c.Logger.Level) == "" {
		return fmt.Errorf("logger level is empty")
	}
	if c.Logger.Format != "json" && c.Logger.Format != "console" {
		return fmt.Errorf("logger format must be json or console")
	}
	if strings.TrimSpace(c.Logger.Filename) == "" {
		return fmt.Errorf("logger filename is empty")
	}
	if c.Logger.MaxSize <= 0 || c.Logger.MaxBackups < 0 || c.Logger.MaxAge < 0 {
		return fmt.Errorf("logger rotation configuration is invalid")
	}

	if c.Tracing.Enabled {
		if strings.TrimSpace(c.Tracing.ServiceName) == "" || strings.TrimSpace(c.Tracing.Endpoint) == "" {
			return fmt.Errorf("tracing service name and endpoint are required")
		}
		if err := validateHostPort("tracing collector", c.Tracing.Endpoint); err != nil {
			return err
		}
		if c.Tracing.SamplerParam < 0 || c.Tracing.SamplerParam > 1 {
			return fmt.Errorf("tracing sampler parameter must be between 0 and 1")
		}
		if c.Tracing.UseTLS && (strings.TrimSpace(c.Tracing.CAFile) == "" ||
			strings.TrimSpace(c.Tracing.CertFile) == "" || strings.TrimSpace(c.Tracing.KeyFile) == "" ||
			strings.TrimSpace(c.Tracing.ServerName) == "") {
			return fmt.Errorf("tracing CA, certificate, private key, and server name are required when TLS is enabled")
		}
	}
	if c.RateLimit.Enabled && !c.Redis.Enabled {
		return fmt.Errorf("Redis must be enabled when rate limiting is enabled")
	}
	if err := validateRateLimit(c.RateLimit); err != nil {
		return err
	}
	if err := validatePositiveDuration("shutdown timeout", c.Shutdown.Timeout); err != nil {
		return err
	}
	return nil
}
