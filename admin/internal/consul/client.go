package consul

import (
	"admin/internal/config"
	"context"
	"fmt"
	"gateway/pkg/apperror"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/api"
)

const (
	protocolGRPC = "grpc"
)

type ConsulRegistry struct {
	client     *api.Client
	httpClient *http.Client
	config     config.ConsulConfig
}

// 构造函数
func NewConsulRegistry(config config.ConsulConfig) (*ConsulRegistry, error) {
	// 创建consul配置文件
	consulConfig := api.DefaultConfig()
	addresess := config.GetAddresses()
	if len(addresess) > 0 {
		consulConfig.Address = addresess[0]
	}
	if config.Scheme != "" {
		consulConfig.Scheme = config.Scheme
	}
	if config.Token != "" {
		consulConfig.Token = config.Token
	} // 设置consul token
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create Consul client", http.StatusInternalServerError)
	}
	// 创建consul注册中心
	return &ConsulRegistry{
		client:     client,
		httpClient: consulConfig.HttpClient,
		config:     config,
	}, nil
}

// 设置关闭

// RegisterGRPC 注册 gRPC 服务实例。
// Close 释放 Consul HTTP 客户端维护的空闲连接。
// 它不会从 Consul 注销当前服务，退出前仍然需要调用 DeregisterGRPC。
func (c *ConsulRegistry) Close() {
	if c == nil || c.httpClient == nil {
		return
	}
	c.httpClient.CloseIdleConnections()
}

func (c *ConsulRegistry) RegisterGRPC(name string, host string, port int, userTls bool) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	if name == "" || host == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if port < 1 || port > 65535 {
		return apperror.InvalidArgument(
			"gRPC port must be between 1 and 65535",
		)
	}
	serviceName := name + "-grpc"
	serviceID := grpcServiceID(name, host, port)
	// 创建服务实例
	registerGrpc := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Tags:    []string{protocolGRPC, name},
		Check: &api.AgentServiceCheck{
			GRPC:                           net.JoinHostPort(host, strconv.Itoa(port)),
			GRPCUseTLS:                     userTls,
			Interval:                       c.config.CheckInterval,
			Timeout:                        c.config.CheckTimeout,
			DeregisterCriticalServiceAfter: c.config.DeregisterCriticalAfter,
		},
	}
	// 注册服务实例
	err := c.client.Agent().ServiceRegister(registerGrpc)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to register service", http.StatusInternalServerError)
	}
	return nil
}

func grpcServiceID(name, host string, port int) string {
	return fmt.Sprintf(
		"%s-grpc-%s-%d",
		strings.TrimSpace(name),
		strings.TrimSpace(host),
		port,
	)
}

// 移除服务
func (c *ConsulRegistry) DeregisterGRPC(name, host string, port int) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	if name == "" || host == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if port < 1 || port > 65535 {
		return apperror.InvalidArgument("gRPC port must be between 1 and 65535")
	}

	if err := c.client.Agent().ServiceDeregister(grpcServiceID(name, host, port)); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to deregister gRPC service from Consul", http.StatusServiceUnavailable)
	}
	return nil
}

// ping
func (c *ConsulRegistry) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return apperror.InvalidArgument("consul client is nil")
	}
	options := new(api.QueryOptions).WithContext(ctx)
	leader, err := c.client.Status().LeaderWithQueryOptions(options)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to get Consul leader", http.StatusInternalServerError)
	}
	if strings.TrimSpace(leader) == "" {
		return apperror.Unavailable("consul leader is empty")
	}
	return nil
}
