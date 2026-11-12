// Package config 定义并加载 Web3 服务配置。
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment     string                `mapstructure:"environment"`
	Name            string                `mapstructure:"name"`
	Version         string                `mapstructure:"version"`
	Host            string                `mapstructure:"host"`
	GRPCPort        int                   `mapstructure:"grpc_port"`
	AdvertiseHost   string                `mapstructure:"advertise_host"`
	Database        DatabaseConfig        `mapstructure:"database"`
	Redis           RedisConfig           `mapstructure:"redis"`
	RabbitMQ        RabbitMQConfig        `mapstructure:"rabbitmq"`
	Auth            AuthConfig            `mapstructure:"auth"`
	GRPC            GRPCConfig            `mapstructure:"grpc"`
	Consul          ConsulConfig          `mapstructure:"consul"`
	WalletSignature WalletSignatureConfig `mapstructure:"wallet_signature"`
	Blockchain      BlockchainConfig      `mapstructure:"blockchain"`
	TronApprovals   TronApprovalConfig    `mapstructure:"tron_approvals"`
	Product         ProductConfig         `mapstructure:"product"`
	Logger          LoggerConfig          `mapstructure:"logger"`
	Tracing         TracingConfig         `mapstructure:"tracing"`
	RateLimit       RateLimitConfig       `mapstructure:"rate_limit"`
	Pprof           PprofConfig           `mapstructure:"pprof"`
	Shutdown        ShutdownConfig        `mapstructure:"shutdown"`
}

// ProductConfig 只保存允许公开给客户端的产品配置。
// API Key、预算阈值和内部服务地址不得放入该结构。
type ProductConfig struct {
	AirdropEnabled         bool   `mapstructure:"airdrop_enabled"`
	FavoriteLimit          uint32 `mapstructure:"favorite_limit"`
	MinimumRefreshInterval string `mapstructure:"minimum_refresh_interval"`
	ClientEventRateLimit   int64  `mapstructure:"client_event_rate_limit"`
	ClientEventRateWindow  string `mapstructure:"client_event_rate_window"`
	FeedbackRateLimit      int64  `mapstructure:"feedback_rate_limit"`
	FeedbackRateWindow     string `mapstructure:"feedback_rate_window"`
	SupportEmail           string `mapstructure:"support_email"`
	FeedbackURL            string `mapstructure:"feedback_url"`
	TestFlightURL          string `mapstructure:"testflight_url"`
	PrivacyPolicyVersion   string `mapstructure:"privacy_policy_version"`
	MaintenanceMessage     string `mapstructure:"maintenance_message"`
	UpdatedAt              string `mapstructure:"updated_at"`
}

type DatabaseConfig struct {
	Enabled               bool     `mapstructure:"enabled"`
	Driver                string   `mapstructure:"driver"`
	Host                  string   `mapstructure:"host"`
	Port                  int      `mapstructure:"port"`
	Username              string   `mapstructure:"username"`
	Password              string   `mapstructure:"-"`
	DBName                string   `mapstructure:"dbname"`
	SSLMode               string   `mapstructure:"ssl_mode"`
	TimeZone              string   `mapstructure:"timezone"`
	ConnectTimeout        string   `mapstructure:"connect_timeout"`
	ReadWriteSplitEnabled bool     `mapstructure:"read_write_split_enabled"`
	ReplicaAddresses      []string `mapstructure:"replica_addresses"`
	MaxIdleConns          int      `mapstructure:"max_idle_conns"`
	MaxOpenConns          int      `mapstructure:"max_open_conns"`
	ConnMaxIdleTime       string   `mapstructure:"conn_max_idle_time"`
	ConnMaxLifetime       string   `mapstructure:"conn_max_lifetime"`
}

func (c DatabaseConfig) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

type RedisConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	Host             string   `mapstructure:"host"`
	Port             int      `mapstructure:"port"`
	ClusterAddresses []string `mapstructure:"cluster_addresses"`
	Password         string   `mapstructure:"-"`
	DB               int      `mapstructure:"db"`
	KeyPrefix        string   `mapstructure:"key_prefix"`
	PoolSize         int      `mapstructure:"pool_size"`
	MinIdleConns     int      `mapstructure:"min_idle_conns"`
	MaxIdleConns     int      `mapstructure:"max_idle_conns"`
	ConnMaxIdleTime  string   `mapstructure:"conn_max_idle_time"`
	ConnMaxLifetime  string   `mapstructure:"conn_max_lifetime"`
	DialTimeout      string   `mapstructure:"dial_timeout"`
	ReadTimeout      string   `mapstructure:"read_timeout"`
	WriteTimeout     string   `mapstructure:"write_timeout"`
	PoolTimeout      string   `mapstructure:"pool_timeout"`
	HealthRequired   bool     `mapstructure:"health_required"`
}

func (c RedisConfig) GetAddresses() []string {
	if len(c.ClusterAddresses) > 0 {
		return append([]string(nil), c.ClusterAddresses...)
	}
	return []string{net.JoinHostPort(c.Host, strconv.Itoa(c.Port))}
}

type RabbitMQConfig struct {
	Enabled               bool                `mapstructure:"enabled"`
	Host                  string              `mapstructure:"host"`
	Port                  int                 `mapstructure:"port"`
	Username              string              `mapstructure:"username"`
	Password              string              `mapstructure:"-"`
	VHost                 string              `mapstructure:"vhost"`
	ConnectionName        string              `mapstructure:"connection_name"`
	Heartbeat             string              `mapstructure:"heartbeat"`
	DialTimeout           string              `mapstructure:"dial_timeout"`
	ReconnectInterval     string              `mapstructure:"reconnect_interval"`
	PublishConfirmTimeout string              `mapstructure:"publish_confirm_timeout"`
	PrefetchCount         int                 `mapstructure:"prefetch_count"`
	ChainEvent            RabbitMQEventConfig `mapstructure:"chain_event"`
}

type RabbitMQEventConfig struct {
	Exchange             string `mapstructure:"exchange"`
	ExchangeType         string `mapstructure:"exchange_type"`
	RoutingKey           string `mapstructure:"routing_key"`
	Queue                string `mapstructure:"queue"`
	RetryExchange        string `mapstructure:"retry_exchange"`
	RetryRoutingKey      string `mapstructure:"retry_routing_key"`
	RetryQueue           string `mapstructure:"retry_queue"`
	RetryDelay           string `mapstructure:"retry_delay"`
	MaxRetries           int    `mapstructure:"max_retries"`
	DeadLetterExchange   string `mapstructure:"dead_letter_exchange"`
	DeadLetterRoutingKey string `mapstructure:"dead_letter_routing_key"`
	DeadLetterQueue      string `mapstructure:"dead_letter_queue"`
}

type AuthConfig struct {
	AccessToken AccessTokenConfig `mapstructure:"access_token"`
}

type AccessTokenConfig struct {
	Issuer       string   `mapstructure:"issuer"`
	Audience     string   `mapstructure:"audience"`
	Algorithm    string   `mapstructure:"algorithm"`
	JWKSURL      string   `mapstructure:"jwks_url"`
	RolePriority []string `mapstructure:"role_priority"`
}

type GRPCConfig struct {
	UseTLS             bool             `mapstructure:"use_tls"`
	TLSServerName      string           `mapstructure:"tls_server_name"`
	CertFile           string           `mapstructure:"cert_file"`
	KeyFile            string           `mapstructure:"key_file"`
	ClientCAFile       string           `mapstructure:"client_ca_file"`
	Health             GRPCHealthConfig `mapstructure:"health"`
	MaxRecvMessageSize int              `mapstructure:"max_recv_message_size"`
	MaxSendMessageSize int              `mapstructure:"max_send_message_size"`
}

type GRPCHealthConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Service string `mapstructure:"service"`
}

type ConsulConfig struct {
	Enabled                 bool                          `mapstructure:"enabled"`
	Host                    string                        `mapstructure:"host"`
	Port                    int                           `mapstructure:"port"`
	Token                   string                        `mapstructure:"-"`
	Scheme                  string                        `mapstructure:"scheme"`
	CheckHost               string                        `mapstructure:"check_host"`
	CheckInterval           string                        `mapstructure:"check_interval"`
	CheckTimeout            string                        `mapstructure:"check_timeout"`
	DeregisterCriticalAfter string                        `mapstructure:"deregister_critical_after"`
	RuntimeConfigKey        string                        `mapstructure:"runtime_config_key"`
	RuntimeConfigRequired   bool                          `mapstructure:"runtime_config_required"`
	RegistrationRequired    bool                          `mapstructure:"registration_required"`
	DeregisterOnShutdown    bool                          `mapstructure:"deregister_on_shutdown"`
	RegistrationMaintenance RegistrationMaintenanceConfig `mapstructure:"registration_maintenance"`
}

func (c ConsulConfig) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

type RegistrationMaintenanceConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Interval   string `mapstructure:"interval"`
	MaxBackoff string `mapstructure:"max_backoff"`
}

type WalletSignatureConfig struct {
	Domain           string `mapstructure:"domain"`
	URI              string `mapstructure:"uri"`
	Statement        string `mapstructure:"statement"`
	NonceTTL         string `mapstructure:"nonce_ttl"`
	NonceBytes       int    `mapstructure:"nonce_bytes"`
	AllowedClockSkew string `mapstructure:"allowed_clock_skew"`
}

type BlockchainConfig struct {
	RequestTimeout  string                  `mapstructure:"request_timeout"`
	RetryCount      int                     `mapstructure:"retry_count"`
	BalanceCacheTTL string                  `mapstructure:"balance_cache_ttl"`
	Scanner         ScannerConfig           `mapstructure:"scanner"`
	Chains          []BlockchainChainConfig `mapstructure:"chains"`
}

type ScannerConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	PollInterval string `mapstructure:"poll_interval"`
	BatchSize    int    `mapstructure:"batch_size"`
	StartBlock   uint64 `mapstructure:"start_block"`
}

type BlockchainChainConfig struct {
	//链路ID
	ChainID               uint64   `mapstructure:"chain_id"`
	Name                  string   `mapstructure:"name"`
	Enabled               bool     `mapstructure:"enabled"`
	RPCURLs               []string `mapstructure:"rpc_urls"`
	NativeSymbol          string   `mapstructure:"native_symbol"`
	NativeDecimals        int      `mapstructure:"native_decimals"`
	RequiredConfirmations uint64   `mapstructure:"required_confirmations"`
}

// TronApprovalConfig 配置 TRON 主网 USDT 授权体检所使用的只读数据源。
// APIKey 只允许通过 WEB3_TRON_API_KEY 注入，不能写入 YAML 或提交到仓库。
type TronApprovalConfig struct {
	Enabled                bool   `mapstructure:"enabled"`
	TronScanBaseURL        string `mapstructure:"tronscan_base_url"`
	TronGridBaseURL        string `mapstructure:"trongrid_base_url"`
	APIKey                 string `mapstructure:"-"`
	USDTContract           string `mapstructure:"usdt_contract"`
	CacheTTL               string `mapstructure:"cache_ttl"`
	RequestTimeout         string `mapstructure:"request_timeout"`
	PageSize               int    `mapstructure:"page_size"`
	MaxPages               int    `mapstructure:"max_pages"`
	MaxSpenders            int    `mapstructure:"max_spenders"`
	VerifyWorkers          int    `mapstructure:"verify_workers"`
	UsageHMACKey           string `mapstructure:"-"`
	DailyQueryLimit        int64  `mapstructure:"daily_query_limit"`
	DailyAddressLimit      int64  `mapstructure:"daily_distinct_address_limit"`
	DailyUpstreamCallLimit int64  `mapstructure:"daily_upstream_call_limit"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	ServiceName  string  `mapstructure:"service_name"`
	Endpoint     string  `mapstructure:"endpoint"`
	SamplerType  string  `mapstructure:"sampler_type"`
	SamplerParam float64 `mapstructure:"sampler_param"`
	UseTLS       bool    `mapstructure:"use_tls"`
	CAFile       string  `mapstructure:"ca_file"`
	CertFile     string  `mapstructure:"cert_file"`
	KeyFile      string  `mapstructure:"key_file"`
	ServerName   string  `mapstructure:"server_name"`
}

type RateLimitConfig struct {
	Enabled              bool    `mapstructure:"enabled"`
	RequestsPerSecond    float64 `mapstructure:"requests_per_second"`
	Burst                int     `mapstructure:"burst"`
	MaxUnaryConcurrency  int     `mapstructure:"max_unary_concurrency"`
	MaxStreamConcurrency int     `mapstructure:"max_stream_concurrency"`
}

type PprofConfig struct {
	Enabled              bool   `mapstructure:"enabled"`
	Host                 string `mapstructure:"host"`
	Port                 int    `mapstructure:"port"`
	AllowRemote          bool   `mapstructure:"allow_remote"`
	BlockProfileRate     int    `mapstructure:"block_profile_rate"`
	MutexProfileFraction int    `mapstructure:"mutex_profile_fraction"`
}

type ShutdownConfig struct {
	Timeout string `mapstructure:"timeout"`
}

// InitConfig 从 YAML 加载配置，随后注入环境变量中的密钥并执行校验。
func InitConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	if strings.TrimSpace(configPath) == "" {
		v.SetConfigName("web3")
		v.SetConfigType("yaml")
		v.AddConfigPath("./etc")
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read Web3 config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode Web3 config: %w", err)
	}
	applyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyEnvironment(cfg *Config) {
	if value := os.Getenv("WEB3_ENVIRONMENT"); value != "" {
		cfg.Environment = value
	}
	if value := os.Getenv("WEB3_ADVERTISE_HOST"); value != "" {
		cfg.AdvertiseHost = value
	}
	if value := os.Getenv("WEB3_DB_HOST"); value != "" {
		cfg.Database.Host = value
	}
	if value := os.Getenv("WEB3_DB_PASSWORD"); value != "" {
		cfg.Database.Password = value
	}
	if value := os.Getenv("WEB3_REDIS_HOST"); value != "" {
		cfg.Redis.Host = value
	}
	if value := os.Getenv("WEB3_REDIS_PASSWORD"); value != "" {
		cfg.Redis.Password = value
	}
	if value := os.Getenv("WEB3_RABBITMQ_HOST"); value != "" {
		cfg.RabbitMQ.Host = value
	}
	if value := os.Getenv("WEB3_RABBITMQ_PASSWORD"); value != "" {
		cfg.RabbitMQ.Password = value
	}
	if value := os.Getenv("WEB3_AUTH_ISSUER"); value != "" {
		cfg.Auth.AccessToken.Issuer = value
	}
	if value := os.Getenv("WEB3_AUTH_JWKS_URL"); value != "" {
		cfg.Auth.AccessToken.JWKSURL = value
	}
	if value := os.Getenv("WEB3_CONSUL_HOST"); value != "" {
		cfg.Consul.Host = value
	}
	if value := os.Getenv("WEB3_CONSUL_CHECK_HOST"); value != "" {
		cfg.Consul.CheckHost = value
	}
	if value := os.Getenv("WEB3_CONSUL_TOKEN"); value != "" {
		cfg.Consul.Token = value
	}
	if value := os.Getenv("WEB3_GRPC_CERT_FILE"); value != "" {
		cfg.GRPC.CertFile = value
	}
	if value := os.Getenv("WEB3_GRPC_KEY_FILE"); value != "" {
		cfg.GRPC.KeyFile = value
	}
	if value := os.Getenv("WEB3_GRPC_CA_FILE"); value != "" {
		cfg.GRPC.ClientCAFile = value
	}
	if value := os.Getenv("WEB3_TRACING_ENDPOINT"); value != "" {
		cfg.Tracing.Endpoint = value
	}
	if value := os.Getenv("WEB3_TRON_API_KEY"); value != "" {
		cfg.TronApprovals.APIKey = value
	}
	if value := os.Getenv("WEB3_TRON_USAGE_HMAC_KEY"); value != "" {
		cfg.TronApprovals.UsageHMACKey = value
	}
	for i := range cfg.Blockchain.Chains {
		key := "WEB3_CHAIN_" + strconv.FormatUint(cfg.Blockchain.Chains[i].ChainID, 10) + "_RPC_URL"
		if value := os.Getenv(key); value != "" {
			cfg.Blockchain.Chains[i].RPCURLs = []string{value}
		}
	}
}

func positiveDuration(name, value string) error {
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
	}
	return nil
}

func validPort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s port %d is outside 1..65535", name, port)
	}
	return nil
}

func validOptionalHTTPSURL(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL", name)
	}
	return nil
}

// Validate 在创建数据库、Redis、消息队列等长期依赖前检查配置。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("Web3 service name is empty")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("Web3 service host is empty")
	}
	if err := validPort("Web3 gRPC", c.GRPCPort); err != nil {
		return err
	}
	if strings.TrimSpace(c.AdvertiseHost) == "" || c.AdvertiseHost == "0.0.0.0" {
		return fmt.Errorf("Web3 advertise host must be reachable")
	}

	if c.Database.Enabled {
		if strings.ToLower(strings.TrimSpace(c.Database.Driver)) != "postgres" {
			return fmt.Errorf("Web3 database driver must be postgres")
		}
		if strings.TrimSpace(c.Database.Host) == "" || strings.TrimSpace(c.Database.Username) == "" || strings.TrimSpace(c.Database.DBName) == "" {
			return fmt.Errorf("Web3 database configuration is incomplete")
		}
		if err := validPort("database", c.Database.Port); err != nil {
			return err
		}
		if c.Database.MaxOpenConns <= 0 || c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
			return fmt.Errorf("database connection pool configuration is invalid")
		}
		for name, value := range map[string]string{"database connect timeout": c.Database.ConnectTimeout, "database max idle time": c.Database.ConnMaxIdleTime, "database max lifetime": c.Database.ConnMaxLifetime} {
			if err := positiveDuration(name, value); err != nil {
				return err
			}
		}
		if c.Database.ReadWriteSplitEnabled && len(c.Database.ReplicaAddresses) == 0 {
			return fmt.Errorf("database replicas are required when read/write splitting is enabled")
		}
	}

	if c.Redis.Enabled {
		for _, address := range c.Redis.GetAddresses() {
			if _, _, err := net.SplitHostPort(address); err != nil {
				return fmt.Errorf("invalid Redis address %q: %w", address, err)
			}
		}
		if strings.TrimSpace(c.Redis.KeyPrefix) == "" {
			return fmt.Errorf("Redis key prefix is empty")
		}
		if c.Redis.PoolSize <= 0 || c.Redis.MinIdleConns < 0 || c.Redis.MaxIdleConns < 0 {
			return fmt.Errorf("Redis pool configuration is invalid")
		}
		for name, value := range map[string]string{"Redis dial timeout": c.Redis.DialTimeout, "Redis read timeout": c.Redis.ReadTimeout, "Redis write timeout": c.Redis.WriteTimeout, "Redis pool timeout": c.Redis.PoolTimeout, "Redis max idle time": c.Redis.ConnMaxIdleTime, "Redis max lifetime": c.Redis.ConnMaxLifetime} {
			if err := positiveDuration(name, value); err != nil {
				return err
			}
		}
	}

	if c.RabbitMQ.Enabled {
		if strings.TrimSpace(c.RabbitMQ.Host) == "" || strings.TrimSpace(c.RabbitMQ.Username) == "" || strings.TrimSpace(c.RabbitMQ.VHost) == "" {
			return fmt.Errorf("RabbitMQ connection configuration is incomplete")
		}
		if err := validPort("RabbitMQ", c.RabbitMQ.Port); err != nil {
			return err
		}
		if c.RabbitMQ.PrefetchCount <= 0 {
			return fmt.Errorf("RabbitMQ prefetch count must be positive")
		}
		for name, value := range map[string]string{"RabbitMQ heartbeat": c.RabbitMQ.Heartbeat, "RabbitMQ dial timeout": c.RabbitMQ.DialTimeout, "RabbitMQ reconnect interval": c.RabbitMQ.ReconnectInterval, "RabbitMQ publish confirm timeout": c.RabbitMQ.PublishConfirmTimeout, "RabbitMQ retry delay": c.RabbitMQ.ChainEvent.RetryDelay} {
			if err := positiveDuration(name, value); err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(c.Auth.AccessToken.Issuer) == "" || strings.TrimSpace(c.Auth.AccessToken.Audience) == "" || strings.TrimSpace(c.Auth.AccessToken.JWKSURL) == "" {
		return fmt.Errorf("Web3 access-token configuration is incomplete")
	}
	if c.Auth.AccessToken.Algorithm != "RS256" {
		return fmt.Errorf("Web3 access-token algorithm must be RS256")
	}
	if c.GRPC.UseTLS && (c.GRPC.CertFile == "" || c.GRPC.KeyFile == "") {
		return fmt.Errorf("gRPC certificate and private key are required when TLS is enabled")
	}
	if c.Consul.Enabled {
		if strings.TrimSpace(c.Consul.Host) == "" {
			return fmt.Errorf("Consul host is empty")
		}
		if err := validPort("Consul", c.Consul.Port); err != nil {
			return err
		}
		scheme := strings.ToLower(strings.TrimSpace(c.Consul.Scheme))
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("Consul scheme must be http or https")
		}
		for name, value := range map[string]string{
			"Consul check interval":            c.Consul.CheckInterval,
			"Consul check timeout":             c.Consul.CheckTimeout,
			"Consul critical deregister delay": c.Consul.DeregisterCriticalAfter,
		} {
			if err := positiveDuration(name, value); err != nil {
				return err
			}
		}
		if c.Consul.RuntimeConfigRequired && strings.TrimSpace(c.Consul.RuntimeConfigKey) == "" {
			return fmt.Errorf("Consul runtime config key is required")
		}
		if c.Consul.RegistrationMaintenance.Enabled {
			interval, err := time.ParseDuration(c.Consul.RegistrationMaintenance.Interval)
			if err != nil || interval <= 0 {
				return fmt.Errorf("Consul registration maintenance interval must be positive")
			}
			maxBackoff, err := time.ParseDuration(c.Consul.RegistrationMaintenance.MaxBackoff)
			if err != nil || maxBackoff < interval {
				return fmt.Errorf("Consul registration maintenance max backoff must not be shorter than its interval")
			}
		}
	}

	for name, value := range map[string]string{"wallet nonce TTL": c.WalletSignature.NonceTTL, "wallet clock skew": c.WalletSignature.AllowedClockSkew, "blockchain request timeout": c.Blockchain.RequestTimeout, "balance cache TTL": c.Blockchain.BalanceCacheTTL, "scanner poll interval": c.Blockchain.Scanner.PollInterval, "shutdown timeout": c.Shutdown.Timeout} {
		if err := positiveDuration(name, value); err != nil {
			return err
		}
	}
	if strings.TrimSpace(c.WalletSignature.Domain) == "" ||
		strings.TrimSpace(c.WalletSignature.URI) == "" ||
		strings.TrimSpace(c.WalletSignature.Statement) == "" {
		return fmt.Errorf("wallet signature domain, URI, and statement are required")
	}
	if c.WalletSignature.NonceBytes < 16 {
		return fmt.Errorf("wallet nonce must contain at least 16 random bytes")
	}
	if c.Blockchain.RetryCount < 0 {
		return fmt.Errorf("blockchain retry count cannot be negative")
	}
	if c.Blockchain.Scanner.Enabled && c.Blockchain.Scanner.BatchSize <= 0 {
		return fmt.Errorf("blockchain scanner batch size must be positive")
	}
	if c.TronApprovals.Enabled {
		if strings.TrimSpace(c.TronApprovals.TronScanBaseURL) == "" ||
			strings.TrimSpace(c.TronApprovals.TronGridBaseURL) == "" ||
			strings.TrimSpace(c.TronApprovals.USDTContract) == "" {
			return fmt.Errorf("TRON approval upstream configuration is incomplete")
		}
		if strings.TrimSpace(c.TronApprovals.APIKey) == "" {
			return fmt.Errorf("WEB3_TRON_API_KEY is required when TRON approval queries are enabled")
		}
		if len(c.TronApprovals.UsageHMACKey) < 32 {
			return fmt.Errorf("WEB3_TRON_USAGE_HMAC_KEY must contain at least 32 characters")
		}
		if err := positiveDuration("TRON approval cache TTL", c.TronApprovals.CacheTTL); err != nil {
			return err
		}
		if err := positiveDuration("TRON approval request timeout", c.TronApprovals.RequestTimeout); err != nil {
			return err
		}
		if c.TronApprovals.PageSize < 1 || c.TronApprovals.PageSize > 50 ||
			c.TronApprovals.MaxPages < 1 || c.TronApprovals.MaxSpenders < 1 ||
			c.TronApprovals.VerifyWorkers < 1 {
			return fmt.Errorf("TRON approval query limits are invalid")
		}
		if c.TronApprovals.DailyQueryLimit <= 0 ||
			c.TronApprovals.DailyAddressLimit <= 0 ||
			c.TronApprovals.DailyAddressLimit > c.TronApprovals.DailyQueryLimit ||
			c.TronApprovals.DailyUpstreamCallLimit <= 0 {
			return fmt.Errorf("TRON approval daily budget limits are invalid")
		}
	}
	if c.Product.FavoriteLimit == 0 {
		return fmt.Errorf("product favorite limit must be positive")
	}
	if err := positiveDuration("product minimum refresh interval", c.Product.MinimumRefreshInterval); err != nil {
		return err
	}
	if c.Product.ClientEventRateLimit <= 0 {
		return fmt.Errorf("product client-event rate limit must be positive")
	}
	if err := positiveDuration("product client-event rate window", c.Product.ClientEventRateWindow); err != nil {
		return err
	}
	if c.Product.FeedbackRateLimit <= 0 {
		return fmt.Errorf("product feedback rate limit must be positive")
	}
	if err := positiveDuration("product feedback rate window", c.Product.FeedbackRateWindow); err != nil {
		return err
	}
	if err := validOptionalHTTPSURL("product feedback URL", c.Product.FeedbackURL); err != nil {
		return err
	}
	if err := validOptionalHTTPSURL("product TestFlight URL", c.Product.TestFlightURL); err != nil {
		return err
	}
	if strings.TrimSpace(c.Product.PrivacyPolicyVersion) == "" {
		return fmt.Errorf("product privacy policy version is empty")
	}
	if _, err := time.Parse(time.RFC3339, c.Product.UpdatedAt); err != nil {
		return fmt.Errorf("product updated_at must use RFC3339: %w", err)
	}
	seenChains := make(map[uint64]struct{}, len(c.Blockchain.Chains))
	for _, chain := range c.Blockchain.Chains {
		if chain.ChainID == 0 || strings.TrimSpace(chain.Name) == "" {
			return fmt.Errorf("blockchain chain identity is incomplete")
		}
		if _, ok := seenChains[chain.ChainID]; ok {
			return fmt.Errorf("duplicate blockchain chain ID %d", chain.ChainID)
		}
		seenChains[chain.ChainID] = struct{}{}
		if chain.Enabled && len(chain.RPCURLs) == 0 {
			return fmt.Errorf("enabled chain %d has no RPC URL", chain.ChainID)
		}
		if chain.NativeDecimals < 0 || chain.NativeDecimals > 255 {
			return fmt.Errorf("chain %d native decimals are invalid", chain.ChainID)
		}
	}
	if c.RateLimit.Enabled && (c.RateLimit.RequestsPerSecond <= 0 || c.RateLimit.Burst <= 0 || c.RateLimit.MaxUnaryConcurrency <= 0 || c.RateLimit.MaxStreamConcurrency <= 0) {
		return fmt.Errorf("Web3 rate-limit configuration is invalid")
	}
	if c.Pprof.Enabled {
		if strings.TrimSpace(c.Pprof.Host) == "" {
			return fmt.Errorf("pprof host is empty")
		}
		if err := validPort("pprof", c.Pprof.Port); err != nil {
			return err
		}
		if c.Pprof.BlockProfileRate < 0 || c.Pprof.MutexProfileFraction < 0 {
			return fmt.Errorf("pprof block and mutex sampling rates cannot be negative")
		}
	}
	if c.Tracing.Enabled {
		if strings.TrimSpace(c.Tracing.ServiceName) == "" || strings.TrimSpace(c.Tracing.Endpoint) == "" {
			return fmt.Errorf("Web3 tracing configuration is incomplete")
		}
		if c.Tracing.SamplerParam < 0 || c.Tracing.SamplerParam > 1 {
			return fmt.Errorf("tracing sampler parameter must be between 0 and 1")
		}
	}
	return nil
}
