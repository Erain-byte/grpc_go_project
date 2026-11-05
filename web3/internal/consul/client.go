// Package consul 实现 Web3 服务注册、发现和 Consul KV 访问。
package consul

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"web3/internal/config"
	"web3/pkg/apperror"

	"github.com/hashicorp/consul/api"
)

const protocolGRPC = "grpc"

type ConsulRegistry struct {
	registrationMu sync.Mutex
	registrations  map[string]*api.AgentServiceRegistration
	client         *api.Client
	httpClient     *http.Client
	config         config.ConsulConfig
}

func NewConsulRegistry(cfg config.ConsulConfig) (*ConsulRegistry, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = cfg.Address()
	consulConfig.Scheme = cfg.Scheme
	consulConfig.Token = cfg.Token

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create Consul client", http.StatusInternalServerError)
	}
	return &ConsulRegistry{
		registrations: make(map[string]*api.AgentServiceRegistration),
		client:        client,
		httpClient:    consulConfig.HttpClient,
		config:        cfg,
	}, nil
}

func (r *ConsulRegistry) RegisterGRPC(name, host string, port int, useTLS bool, tlsServerName string) error {
	name = strings.TrimSpace(name)
	host = strings.TrimSpace(host)
	tlsServerName = strings.TrimSpace(tlsServerName)
	if name == "" || host == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if port < 1 || port > 65535 {
		return apperror.InvalidArgument("gRPC port must be between 1 and 65535")
	}
	if useTLS && tlsServerName == "" {
		return apperror.InvalidArgument("gRPC TLS server name is empty")
	}
	if !useTLS {
		tlsServerName = ""
	}

	checkHost := strings.TrimSpace(r.config.CheckHost)
	if checkHost == "" {
		checkHost = host
	}
	registration := &api.AgentServiceRegistration{
		ID:      grpcServiceID(name, host, port),
		Name:    name + "-grpc",
		Address: host,
		Port:    port,
		Tags:    []string{protocolGRPC, name},
		Check: &api.AgentServiceCheck{
			GRPC:                           net.JoinHostPort(checkHost, strconv.Itoa(port)),
			GRPCUseTLS:                     useTLS,
			TLSServerName:                  tlsServerName,
			TLSSkipVerify:                  false,
			Interval:                       r.config.CheckInterval,
			Timeout:                        r.config.CheckTimeout,
			DeregisterCriticalServiceAfter: r.config.DeregisterCriticalAfter,
		},
	}
	if err := r.registerAndRemember(registration); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to register gRPC service in Consul", http.StatusServiceUnavailable)
	}
	return nil
}

func (r *ConsulRegistry) DeregisterGRPC(name, host string, port int) error {
	serviceID := grpcServiceID(name, host, port)
	if strings.TrimSpace(name) == "" || strings.TrimSpace(host) == "" {
		return apperror.InvalidArgument("service name or host is empty")
	}
	if port < 1 || port > 65535 {
		return apperror.InvalidArgument("gRPC port must be between 1 and 65535")
	}
	if err := r.client.Agent().ServiceDeregister(serviceID); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to deregister gRPC service from Consul", http.StatusServiceUnavailable)
	}
	r.registrationMu.Lock()
	delete(r.registrations, serviceID)
	r.registrationMu.Unlock()
	return nil
}

func grpcServiceID(name, host string, port int) string {
	return fmt.Sprintf("%s-grpc-%s-%d", strings.TrimSpace(name), strings.TrimSpace(host), port)
}

func (r *ConsulRegistry) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return apperror.Unavailable("Consul client is not initialized")
	}
	leader, err := r.client.Status().LeaderWithQueryOptions(new(api.QueryOptions).WithContext(ctx))
	if err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to get Consul leader", http.StatusServiceUnavailable)
	}
	if strings.TrimSpace(leader) == "" {
		return apperror.Unavailable("Consul leader is empty")
	}
	return nil
}

func (r *ConsulRegistry) Close() {
	if r != nil && r.httpClient != nil {
		r.httpClient.CloseIdleConnections()
	}
}
