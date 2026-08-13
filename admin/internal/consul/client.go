package consul

import (
	"admin/internal/config"
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
	client *api.Client
	config config.Config
}

// 构造函数
func NewConsulRegistry(config config.Config) (*ConsulRegistry, error) {
	// 创建consul配置文件
	consulConfig := api.DefaultConfig()       // 创建consul客户端
	addresess := config.Consul.GetAddresses() // 获取consul地址
	if len(addresess) > 0 {
		consulConfig.Address = addresess[0] // 设置consul地址
	}
	if config.Consul.Scheme != "" {
		consulConfig.Scheme = config.Consul.Scheme // 设置consul协议
	}
	if config.Consul.Token != "" {
		consulConfig.Token = config.Consul.Token
	} // 设置consul token
	client, err := api.NewClient(consulConfig) // 创建consul客户端
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create Consul client", http.StatusInternalServerError)
	} // 创建consul客户端
	// 创建consul注册中心
	return &ConsulRegistry{
		client: client,
		config: config,
	}, nil
}

// 设置关闭

// RegisterGRPC 注册 gRPC 服务实例。
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
			Interval:                       c.config.Consul.CheckInterval,
			Timeout:                        c.config.Consul.CheckTimeout,
			DeregisterCriticalServiceAfter: c.config.Consul.DeregisterCriticalAfter,
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
