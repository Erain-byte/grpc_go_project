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
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

// \gateway\internal\consul\consul.go
// 定义结构体
type ConsulRegistry struct {
	client          *api.Client
	config          config.ConsulConfig
	cacheMu         sync.RWMutex
	serviceCache    map[string]*serviceCacheEntry //服务缓存
	watchMu         sync.Mutex
	watchedServices map[string]struct{}
	watchCtx        context.Context
	watchCancel     context.CancelFunc
	watchWG         sync.WaitGroup
	closed          bool
}

// serviceCacheEntry 服务缓存条目
type serviceCacheEntry struct {
	entries   []*api.ServiceEntry //服务实例
	lastIndex uint64              //	最后一次更新时间
	updatedAt time.Time
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
	watchCtx, watchCancel := context.WithCancel(context.Background())
	return &ConsulRegistry{
		client:          cli,
		config:          config,
		serviceCache:    make(map[string]*serviceCacheEntry),
		watchedServices: make(map[string]struct{}),
		watchCtx:        watchCtx,
		watchCancel:     watchCancel,
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
	registration := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-http", name),
		Name:    name,
		Address: host,
		Port:    port,
		Tags:    BuildServiceTags(cfg, ProtocolHTTP),
		Meta:    metadata,
		Check: &api.AgentServiceCheck{
			HTTP:                           fmt.Sprintf("%s://%s:%d/health", r.config.Scheme, host, port),
			Interval:                       r.config.CheckInterval,
			Timeout:                        r.config.CheckTimeout, //服务检查间隔
			DeregisterCriticalServiceAfter: r.config.DeregisterCriticalAfter,
			TLSSkipVerify:                  true, //跳过TLS验证
		}, //服务检查
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
	registration := &api.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-grpc", name),
		Name:    name,
		Address: host,
		Port:    port,
		Tags:    BuildServiceTags(cfg, ProtocolGRPC),
		Meta:    metadata,
		Check: &api.AgentServiceCheck{
			GRPC:                           fmt.Sprintf("%s:%d", host, port),
			Interval:                       r.config.CheckInterval,
			Timeout:                        r.config.CheckTimeout, //服务检查间隔
			DeregisterCriticalServiceAfter: r.config.DeregisterCriticalAfter,
		},
	}

	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to register gRPC service with Consul", http.StatusServiceUnavailable)
	}
	logInfof("registered gRPC service %s with Consul", name)
	return nil
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

// DiscoverService 默认发现 HTTP 服务，保留该方法以兼容现有调用。
func (r *ConsulRegistry) DiscoverService(ctx context.Context, name string) ([]*api.ServiceEntry, error) {
	return r.DiscoverHTTPService(ctx, name)
}

// DiscoverHTTPService 发现 HTTP 服务。
func (r *ConsulRegistry) DiscoverHTTPService(ctx context.Context, name string) ([]*api.ServiceEntry, error) {
	return r.discoverService(ctx, name, ProtocolHTTP)
}

// DiscoverGRPCService 发现 gRPC 服务。
func (r *ConsulRegistry) DiscoverGRPCService(ctx context.Context, name string) ([]*api.ServiceEntry, error) {
	return r.discoverService(ctx, name, ProtocolGRPC)
}

// DiscoverService 发现服务。
func (r *ConsulRegistry) discoverService(ctx context.Context, name string, protocol string) ([]*api.ServiceEntry, error) {
	key := serviceCacheKey(name, protocol)
	if entries, found := r.getFromCache(key); found {
		return entries, nil
	}
	entries, lastIndex, err := r.queryConsul(ctx, name, protocol, 0) //查询Consul
	if err != nil {
		return nil, err
	}
	r.updateCache(key, entries, lastIndex)  //更新缓存
	r.startWatch(name, protocol, lastIndex) //启动监听
	return slices.Clone(entries), nil
}

// startWatch 启动服务发现监听
func serviceCacheKey(name string, protocol string) string {
	return strings.TrimSpace(name) + ":" + strings.TrimSpace(protocol)
}

// getFromCache 获取服务实例缓存
func (r *ConsulRegistry) getFromCache(name string) ([]*api.ServiceEntry, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	entry, ok := r.serviceCache[name]
	if !ok {
		return nil, false
	}
	// 检查缓存是否过期（30秒）
	if time.Since(entry.updatedAt) > 30*time.Second {
		return nil, false
	}
	return slices.Clone(entry.entries), true
}

// updateCache 更新服务缓存
func (r *ConsulRegistry) updateCache(name string, entries []*api.ServiceEntry, lastIndex uint64) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()
	r.serviceCache[name] = &serviceCacheEntry{
		entries:   slices.Clone(entries),
		lastIndex: lastIndex,
		updatedAt: time.Now(),
	}
}

// queryConsul 查询 Consul 服务
func (r *ConsulRegistry) queryConsul(ctx context.Context, name string, protocol string, waitIndex uint64) ([]*api.ServiceEntry, uint64, error) {
	if ctx == nil {
		return nil, waitIndex, apperror.InvalidArgument("query consul: context is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, waitIndex, apperror.InvalidArgument("query consul: service name is empty")
	}
	protocol = strings.TrimSpace(protocol)
	if protocol != ProtocolHTTP && protocol != ProtocolGRPC {
		return nil, waitIndex, apperror.InvalidArgument("query consul: protocol must be http or grpc")
	}
	options := (&api.QueryOptions{
		WaitIndex: waitIndex,
		WaitTime:  20 * time.Second, //必须短于当前缓存过期时间
	}).WithContext(ctx) //添加上下文
	entries, meta, err := r.client.Health().Service(name, protocol, true, options) //查询服务
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

// startWatch 保证每个服务在当前 Registry 中最多只有一个 Watch。

func (r *ConsulRegistry) startWatch(name string, protocol string, lastIndex uint64) {
	key := serviceCacheKey(name, protocol)
	r.watchMu.Lock()
	if r.closed {
		r.watchMu.Unlock()
		return
	}
	if _, exists := r.watchedServices[key]; exists {
		r.watchMu.Unlock()
		return
	}
	r.watchedServices[key] = struct{}{}
	r.watchWG.Add(1)
	r.watchMu.Unlock()

	go r.watchService(r.watchCtx, name, protocol, lastIndex)
}

// watchService 后台监听服务变化
func (r *ConsulRegistry) watchService(ctx context.Context, name string, protocol string, lastIndex uint64) {
	key := serviceCacheKey(name, protocol)
	defer r.watchWG.Done()
	defer r.removeWatchService(key)
	for {
		entries, newIndex, err := r.queryConsul(ctx, name, protocol, lastIndex)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logErrorf("failed to watch %s service %s: %v", protocol, name, err)
			if !waitRetry(ctx, 5*time.Second) {
				return
			}
			continue
		}

		if newIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if newIndex == 0 {
			newIndex = 1
		}

		r.updateCache(key, entries, newIndex)
		lastIndex = newIndex
	}
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

// removeWatchService 移除监听服务
func (r *ConsulRegistry) removeWatchService(name string) {
	r.watchMu.Lock()
	defer r.watchMu.Unlock()
	delete(r.watchedServices, name)
}

// Close 停止 Registry 创建的所有 Watch，并等待 goroutine 退出。
func (r *ConsulRegistry) Close() {
	if r == nil {
		return
	}
	r.watchMu.Lock()
	if r.closed {
		r.watchMu.Unlock()
		return
	}
	r.closed = true
	r.watchCancel()
	r.watchMu.Unlock()
	r.watchWG.Wait()
}

// GetServiceMetadata 获取服务的元数据（公开接口、CORS配置等）
func (r *ConsulRegistry) GetServiceMetadata(ctx context.Context, name string) (map[string]string, error) {

	entries, err := r.DiscoverService(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, apperror.NotFound(fmt.Sprintf("service %q not found", name))
	}

	return entries[0].Service.Meta, nil

}

// GetPublicEndpoints 获取服务的公开接口列表
func (r *ConsulRegistry) GetPublicEndpoints(ctx context.Context, name string) ([]string, error) {
	metadata, err := r.GetServiceMetadata(ctx, name)
	if err != nil {
		return nil, err
	}
	if publicAPIs, ok := metadata["public_apis"]; ok { //获取公开接口列表
		//return strings.Split(publicAPIs, ","), nil
		var listAPIs []string
		apiList := strings.Split(publicAPIs, ",")
		for _, api := range apiList {
			listAPIs = append(listAPIs, strings.TrimSpace(api))
		}
		return listAPIs, nil
	}
	return []string{}, nil
}

// DeregisterHTTPService 删除HTTP服务
func (r *ConsulRegistry) DeregisterHTTPService(name string) error {
	return r.deregisterService(name, ProtocolHTTP)
}

// DeregisterGRPCService 删除GRPC服务
func (r *ConsulRegistry) DeregisterGRPCService(name string) error {
	return r.deregisterService(name, ProtocolGRPC)
}

// deregisterService 删除服务
func (r *ConsulRegistry) deregisterService(name string, protocol string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperror.InvalidArgument("service name is empty")
	}
	serviceID := fmt.Sprintf("%s-%s", name, protocol)
	if err := r.client.Agent().ServiceDeregister(serviceID); err != nil {
		return apperror.Wrap(err, apperror.CodeUnavailable, "failed to deregister service from Consul", http.StatusServiceUnavailable)
	}
	logInfof("deregistered service %q", serviceID)
	return nil
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
