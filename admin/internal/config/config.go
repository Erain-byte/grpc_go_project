// Package config defines and loads Admin service configuration.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

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
	Auth        AuthConfig       `yaml:"auth" mapstructure:"auth"`
	Consul      ConsulConfig     `yaml:"consul" mapstructure:"consul"`
	Logger      LoggerConfig     `yaml:"logger" mapstructure:"logger"`
	Shutdown    ShutdownConfig   `yaml:"shutdown" mapstructure:"shutdown"`
	GRPC        GRPCServerConfig `yaml:"grpc" mapstructure:"grpc"`
	Tracing     TracingConfig    `yaml:"tracing" mapstructure:"tracing"`
}

type DatabaseConfig struct {
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
	AccessToken  AccessTokenConfig  `yaml:"access_token" mapstructure:"access_token"`
	RefreshToken RefreshTokenConfig `yaml:"refresh_token" mapstructure:"refresh_token"`
}

type AccessTokenConfig struct {
	Issuer    string `yaml:"issuer" mapstructure:"issuer"`
	Audience  string `yaml:"audience" mapstructure:"audience"`
	Algorithm string `yaml:"algorithm" mapstructure:"algorithm"`
	Expire    string `yaml:"expire" mapstructure:"expire"`
	Secret    string `yaml:"-" mapstructure:"-"`
}

type RefreshTokenConfig struct {
	Expire         string `yaml:"expire" mapstructure:"expire"`
	Rotate         bool   `yaml:"rotate" mapstructure:"rotate"`
	ReuseDetection bool   `yaml:"reuse_detection" mapstructure:"reuse_detection"`
	RedisKeyPrefix string `yaml:"redis_key_prefix" mapstructure:"redis_key_prefix"`
}

type ConsulConfig struct {
	Host                    string   `yaml:"host" mapstructure:"host"`
	Port                    int      `yaml:"port" mapstructure:"port"`
	Addresses               []string `yaml:"addresses" mapstructure:"addresses"`
	Token                   string   `yaml:"-" mapstructure:"-"`
	Scheme                  string   `yaml:"scheme" mapstructure:"scheme"`
	CheckInterval           string   `yaml:"check_interval" mapstructure:"check_interval"`
	CheckTimeout            string   `yaml:"check_timeout" mapstructure:"check_timeout"`
	DeregisterCriticalAfter string   `yaml:"deregister_critical_after" mapstructure:"deregister_critical_after"`
}

func (c ConsulConfig) GetAddresses() []string {
	if len(c.Addresses) > 0 {
		return append([]string(nil), c.Addresses...)
	}
	return []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))}
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
	UseTLS       bool   `yaml:"use_tls" mapstructure:"use_tls"`
	CertFile     string `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile      string `yaml:"key_file" mapstructure:"key_file"`
	ClientCAFile string `yaml:"client_ca_file" mapstructure:"client_ca_file"`
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
	v.SetDefault("database.driver", "mysql")
	v.SetDefault("database.charset", "utf8mb4")
	v.SetDefault("database.parse_time", true)
	v.SetDefault("database.location", "Local")
	v.SetDefault("database.read_write_split_enabled", false)
	v.SetDefault("database.replica_addresses", []string{})
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("consul.host", "127.0.0.1")
	v.SetDefault("consul.port", 8500)
	v.SetDefault("consul.scheme", "http")
	v.SetDefault("shutdown.timeout", "10s")
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
	if value := os.Getenv("ADMIN_CONSUL_TOKEN"); value != "" {
		cfg.Consul.Token = value
	}
	if value := os.Getenv("ADMIN_ACCESS_TOKEN_SECRET"); value != "" {
		cfg.Auth.AccessToken.Secret = value
	}
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
	if strings.TrimSpace(c.Auth.AccessToken.Issuer) == "" {
		return fmt.Errorf("admin access-token issuer is empty")
	}
	if strings.TrimSpace(c.Auth.AccessToken.Audience) == "" {
		return fmt.Errorf("admin access-token audience is empty")
	}
	if c.GRPC.UseTLS && (strings.TrimSpace(c.GRPC.CertFile) == "" || strings.TrimSpace(c.GRPC.KeyFile) == "") {
		return fmt.Errorf("admin gRPC certificate and private key are required when TLS is enabled")
	}
	return nil
}
