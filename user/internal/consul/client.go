package consul

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"user/internal/config"
	"user/pkg/apperror"

	"github.com/hashicorp/consul/api"
)

const (
	protocolGRPC = "grpc"
)

type ConsulRegistry struct {
	registrationMu sync.Mutex
	//保存当前已经注册的全部实列
	registrations map[string]*api.AgentServiceRegistration
	client        *api.Client
	httpClient    *http.Client
	config        config.ConsulConfig
}

func NewConsulRegistry(config config.ConsulConfig) (*ConsulRegistry, error) {
	// 创建consul配置文件
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.Address()

	if config.Scheme != "" {
		consulConfig.Scheme = config.Scheme
	}

	if config.Token != "" {
		consulConfig.Token = config.Token
	}
	//设置http连接池
	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create Consul client", http.StatusInternalServerError)
	}

	return &ConsulRegistry{
		client:     client,
		httpClient: consulConfig.HttpClient,
		config:     config,
	}, nil
}

// 注Grpc册服务
func (c *ConsulRegistry) RegisterService(name string, host string, port int, userTls bool, tlsServerName string) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	tlsServerName = strings.TrimSpace(tlsServerName)

	if name == "" || host == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if !userTls {
		tlsServerName = ""
	}

	if port <= 0 || port > 65535 {
		return apperror.InvalidArgument("port must be between 1 and 65535")
	}
	serviceName := name + "-grpc"
	serviceID := grpcServiceID(name, host, port)
	checkHost := strings.TrimSpace(c.config.CheckHost)
	if checkHost == "" {
		checkHost = host
	}
	// 创建服务实例
	registerGrpc := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Tags:    []string{protocolGRPC},
		Check: &api.AgentServiceCheck{
			GRPC:                           net.JoinHostPort(checkHost, strconv.Itoa(port)),
			GRPCUseTLS:                     userTls,
			TLSServerName:                  tlsServerName,
			TLSSkipVerify:                  false,
			Interval:                       c.config.CheckInterval,
			Timeout:                        c.config.CheckTimeout,
			DeregisterCriticalServiceAfter: c.config.DeregisterCriticalAfter,
		},
	}
	//注册
	err := c.registerAndRemember(registerGrpc)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to register service", http.StatusInternalServerError)
	}
	return err
}

// 关闭服务
func (c *ConsulRegistry) Close() {
	if c == nil || c.httpClient == nil {
		return
	}
	//关闭http连接
	c.httpClient.CloseIdleConnections()
}

// 移除服务
func (c *ConsulRegistry) DeregisterGRPC(name, host string, port int) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	if name == "" || host == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if port <= 0 || port > 65535 {
		return apperror.InvalidArgument("port must be between 1 and 65535")
	}
	if err := c.client.Agent().ServiceDeregister(grpcServiceID(name, host, port)); err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to deregister service", http.StatusInternalServerError)
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

// ping
func (c *ConsulRegistry) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return apperror.InvalidArgument("consul client is nil")
	}
	options := new(api.QueryOptions).WithContext(ctx)
	//检查consul是否可用带ctx 参数控制超时
	leader, err := c.client.Status().LeaderWithQueryOptions(options)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, "failed to get consul leader", http.StatusInternalServerError)
	}
	if strings.TrimSpace(leader) == "" {
		return apperror.Unavailable("consul leader is empty")
	}
	return nil
}

// waitRetry定时器
func waitRetry(ctx context.Context, waitTime time.Duration) bool {
	timer := time.NewTimer(waitTime)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}

}
