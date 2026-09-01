package consul

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/config"
	"gateway/internal/logger"
	"gateway/pkg/apperror"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// \gateway\internal\consul\consul.go
// 定义结构体
type ConsulRegistry struct {
	client *api.Client // Consul 客户端
	config config.ConsulConfig
}

// 构造函数
func NewConsulRegistry(config config.ConsulConfig) (*ConsulRegistry, error) {
	consulConfig := api.DefaultConfig() //创建Consul配置
	// 优先使用集群地址，否则使用单节点地址
	addresses := config.GetAddresses()
	if len(addresses) > 0 {
		consulConfig.Address = addresses[0]
	}
	if config.Token != "" {
		consulConfig.Token = config.Token
	}

	cli, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create Consul client", http.StatusInternalServerError)
	}
	return &ConsulRegistry{
		client: cli,
		config: config,
	}, nil

}

const (
	ProtocolHTTP = "http"
	ProtocolGRPC = "grpc"
)

// RegisterHTTP 注册 HTTP 服务实例。
func (r *ConsulRegistry) RegisterHTTP(name string, host string, port int, cfg *config.Config) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	if name == "" {
		return apperror.InvalidArgument("service name is empty")
	}
	if host == "" {
		return apperror.InvalidArgument("service host is empty")
	}
	if port <= 0 || port > 65535 {
		return apperror.InvalidArgument("HTTP port must be between 1 and 65535")
	}
	metadata, err := BuildServiceMetadata(cfg)
	if err != nil {
		return err
	}
	checkHost := r.healthCheckHost(host)
	registration := &api.AgentServiceRegistration{
		ID:      buildServiceID(name, ProtocolHTTP, host, port),
		Name:    fmt.Sprintf("%s-http", name),
		Address: host,
		Port:    port,
		Tags:    BuildServiceTags(cfg, ProtocolHTTP),
		Meta:    metadata,
		//服务检查
		Check: &api.AgentServiceCheck{
			//實際檢查地址
			HTTP:     fmt.Sprintf("%s://%s:%d/health", r.config.Scheme, checkHost, port),
			Interval: r.config.CheckInterval,
			//服务检查间隔
			Timeout:                        r.config.CheckTimeout,
			DeregisterCriticalServiceAfter: r.config.DeregisterCriticalAfter,
			//跳过TLS验证
			TLSSkipVerify: true,
		},
	}

	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to register HTTP service with Consul", http.StatusServiceUnavailable)
	}
	logInfof("registered HTTP service %s with Consul", name)
	return nil
}

// RegisterGRPC 注册 gRPC 服务实例。
func (r *ConsulRegistry) RegisterGRPC(name string, host string, port int, cfg *config.Config) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	if name == "" {
		return apperror.InvalidArgument("service name is empty")
	}
	if host == "" {
		return apperror.InvalidArgument("service host is empty")
	}
	if port <= 0 || port > 65535 {
		return apperror.InvalidArgument("gRPC port must be between 1 and 65535")
	}
	metadata, err := BuildServiceMetadata(cfg)
	if err != nil {
		return err
	}
	checkHost := r.healthCheckHost(host)
	registration := &api.AgentServiceRegistration{
		ID:      buildServiceID(name, ProtocolGRPC, host, port),
		Name:    fmt.Sprintf("%s-grpc", name),
		Address: host,
		Port:    port,
		Tags:    BuildServiceTags(cfg, ProtocolGRPC),
		Meta:    metadata,
		Check: &api.AgentServiceCheck{
			GRPC:     fmt.Sprintf("%s:%d", checkHost, port),
			Interval: r.config.CheckInterval,
			//服务检查间隔
			Timeout:                        r.config.CheckTimeout,
			DeregisterCriticalServiceAfter: r.config.DeregisterCriticalAfter,
		},
	}

	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to register gRPC service with Consul", http.StatusServiceUnavailable)
	}
	logInfof("registered gRPC service %s with Consul", name)
	return nil
}

// healthCheckHost 返回 Consul Agent 执行健康检查时使用的主机名。
// Consul 在 Docker 中运行时，127.0.0.1 指向容器自身，需要通过
// host.docker.internal 访问运行在 Windows 主机上的 Go 服务。
func (r *ConsulRegistry) healthCheckHost(serviceHost string) string {
	if checkHost := strings.TrimSpace(r.config.CheckHost); checkHost != "" {
		return checkHost
	}
	return serviceHost
}

// BuildServiceMetadata 构建Gateway服务元数据
func BuildServiceMetadata(cfg *config.Config) (map[string]string, error) {
	if cfg == nil {
		return map[string]string{}, apperror.InvalidArgument("service config is nil")
	}
	publicAPIs := strings.Join(cfg.Service.PublicAPIs, ",")
	authAPIs := strings.Join(cfg.Service.AuthAPIs, ",")
	return map[string]string{
		"public_apis":  publicAPIs,
		"auth_apis":    authAPIs,
		"service-type": cfg.Name,
		"version":      cfg.Service.Version,
	}, nil

}

// BuildServiceTags 构建服务标签
func BuildServiceTags(cfg *config.Config, protocol string) []string {
	tags := make([]string, 0, len(cfg.Service.CTags)+1)
	for _, tag := range cfg.Service.CTags {
		if tag != ProtocolHTTP && tag != ProtocolGRPC {
			tags = append(tags, tag)
		}
	}
	return append(tags, protocol)

}

// queryGRPCService 使用 Consul Blocking Query 查询健康的 gRPC 服务实例。
// waitIndex 为上一次查询返回的 Consul Index；Consul 会在实例变化或 WaitTime
// 到期后返回，从而避免客户端定时轮询。
func (r *ConsulRegistry) queryGRPCService(ctx context.Context, name string, waitIndex uint64) ([]*api.ServiceEntry, uint64, error) {
	if ctx == nil {
		return nil, waitIndex, apperror.InvalidArgument("query consul: context is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, waitIndex, apperror.InvalidArgument("query consul: service name is empty")
	}
	options := (&api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  20 * time.Second,
	}).WithContext(ctx)
	entries, meta, err := r.client.Health().Service(name, ProtocolGRPC, true, options)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, waitIndex, apperror.Wrap(err, apperror.CodeTimeout, "Consul query timed out", http.StatusGatewayTimeout)
		}
		return nil, waitIndex, apperror.Wrap(err, apperror.CodeUnavailable, "failed to query service from Consul", http.StatusServiceUnavailable)
	}
	if meta == nil {
		return nil, waitIndex, apperror.Unavailable("Consul query returned no metadata")
	}
	return entries, meta.LastIndex, nil
}

// WatchGRPCService 持续监听健康的 gRPC 服务实例，并在地址发生查询更新时回调。
// 重连和退避由 grpcResolver 统一负责，Registry 只负责一次连续的 Blocking Query。
func (r *ConsulRegistry) WatchGRPCService(ctx context.Context, name string, onUpdate func([]*api.ServiceEntry) error) error {
	if ctx == nil {
		return apperror.InvalidArgument("watch grpc service: context is nil")
	}
	if strings.TrimSpace(name) == "" {
		return apperror.InvalidArgument("watch grpc service: service name is empty")
	}
	if onUpdate == nil {
		return apperror.InvalidArgument("watch grpc service: onUpdate is nil")
	}
	var lastIndex uint64
	for {
		entries, newIndex, err := r.queryGRPCService(ctx, name, lastIndex)

		if err != nil {
			return err
		}
		if newIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if newIndex == 0 {
			newIndex = 1
		}
		if err := onUpdate(slices.Clone(entries)); err != nil {
			return err
		}
		lastIndex = newIndex
	}

}

// ping
func (r *ConsulRegistry) Ping(ctx context.Context) error {
	if r.client == nil {
		return errors.New("conusl client is nil")
	}
	options := new(api.QueryOptions).WithContext(ctx)

	leader, err := r.client.Status().LeaderWithQueryOptions(options)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"Consul is unavailable",
			http.StatusServiceUnavailable,
		)
	}

	if strings.TrimSpace(leader) == "" {
		return apperror.Unavailable(
			"Consul cluster has no leader",
		)
	}
	return nil
}
func waitRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// DeregisterHTTPService 删除HTTP服务
func (r *ConsulRegistry) DeregisterHTTPService(name, host string, port int) error {
	return r.deregisterService(name, ProtocolHTTP, host, port)
}

// DeregisterGRPCService 删除GRPC服务
func (r *ConsulRegistry) DeregisterGRPCService(name, host string, port int) error {
	return r.deregisterService(name, ProtocolGRPC, host, port)
}

// deregisterService 删除服务
func (r *ConsulRegistry) deregisterService(name string, protocol string, host string, port int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperror.InvalidArgument("service name is empty")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return apperror.InvalidArgument("service host is empty")
	}
	if port <= 0 || port > 65535 {
		return apperror.InvalidArgument("service port must be between 1 and 65535")
	}
	serviceID := buildServiceID(name, protocol, host, port)
	if err := r.client.Agent().ServiceDeregister(serviceID); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to deregister service from Consul", http.StatusServiceUnavailable)
	}
	logInfof("deregistered service %q", serviceID)
	return nil
}

func buildServiceID(name, protocol, host string, port int) string {
	return fmt.Sprintf("%s-%s-%s-%d", name, protocol, host, port)
}

// logInfo
func logInfof(template string, args ...any) {
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof(template, args...)
	}
}

// logError
func logErrorf(template string, args ...any) {
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Errorf(template, args...)
	}
}
