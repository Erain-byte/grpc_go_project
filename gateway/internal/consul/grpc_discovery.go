// Package consul 提供 Consul 服务注册、服务发现及与 grpc-go 的适配能力。
package consul

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc/resolver"
)

const (
	grpcResolverScheme   = "consul"
	grpcRefreshPeriod    = 5 * time.Second
	grpcDiscoveryTimeout = 5 * time.Second
)

// GRPCServiceDiscoverer 定义 gRPC resolver 所需的最小服务发现能力。
// 使用接口后，ClientManager 和 resolver 不必依赖具体的 ConsulRegistry，测试时也可以传入假实现。
type GRPCServiceDiscoverer interface {
	DiscoverGRPCService(context.Context, string) ([]*api.ServiceEntry, error)
}

// DiscoverGRPCService 查询通过健康检查的 gRPC 服务实例。
// 底层复用通用 discoverService，并通过 ProtocolGRPC 限定协议类型。
func (r *ConsulRegistry) DiscoverGRPCService(ctx context.Context, name string) ([]*api.ServiceEntry, error) {
	return r.discoverService(ctx, name, ProtocolGRPC)
}

// GRPCResolverBuilder 是 Consul 服务发现与 grpc-go resolver 之间的适配器。
// grpc.NewClient 会根据 Scheme 找到它，再调用 Build 创建一个服务专用的 resolver。
type GRPCResolverBuilder struct {
	discoverer GRPCServiceDiscoverer
}

// NewGRPCResolverBuilder 创建 Consul gRPC resolver 构建器。
func NewGRPCResolverBuilder(discoverer GRPCServiceDiscoverer) *GRPCResolverBuilder {
	return &GRPCResolverBuilder{discoverer: discoverer}
}

// Scheme 返回 resolver 的协议名，对应 target 中的 consul:// 前缀。
func (b *GRPCResolverBuilder) Scheme() string { return grpcResolverScheme }

// Build 根据逻辑服务名创建 resolver。
// target.Endpoint() 从 consul:///admin-service-grpc 中得到 admin-service-grpc；
// cc 是 grpc-go 提供的回调接口，resolver 通过它把最新地址列表交还给 grpc.ClientConn。
func (b *GRPCResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	if b.discoverer == nil {
		return nil, fmt.Errorf("Consul service discoverer is nil")
	}
	serviceName := strings.TrimSpace(target.Endpoint())
	if serviceName == "" {
		return nil, fmt.Errorf("gRPC resolver service name is empty")
	}

	// 该 context 控制 resolver 的完整生命周期，Close 会调用 cancel 结束后台 watch。
	ctx, cancel := context.WithCancel(context.Background())
	r := &grpcResolver{
		discoverer:  b.discoverer,
		clientConn:  cc,
		serviceName: serviceName,
		ctx:         ctx,
		cancel:      cancel,
		// 容量为 1，用于合并短时间内重复到达的立即刷新请求。
		resolveNow: make(chan struct{}, 1),
	}
	// 第一次发现同步执行，确保 resolver 创建成功时已经向 gRPC 提供了初始地址。
	if err := r.resolve(); err != nil {
		cancel()
		return nil, err
	}
	go r.watch()
	return r, nil
}

// grpcResolver 负责一个逻辑 gRPC 服务的持续地址解析。
type grpcResolver struct {
	discoverer  GRPCServiceDiscoverer // 从 Consul 获取健康实例。
	clientConn  resolver.ClientConn   // 把地址或错误通知给 grpc-go。
	serviceName string                // 例如 admin-service-grpc。
	ctx         context.Context       // resolver 生命周期 context。
	cancel      context.CancelFunc    // 关闭 resolver 及后台 goroutine。
	resolveNow  chan struct{}         // grpc-go 发起的立即刷新信号。
}

// ResolveNow 接收 grpc-go 的立即重新解析请求。
// 使用非阻塞发送：如果已有刷新信号尚未处理，则无需重复排队。
func (r *grpcResolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case r.resolveNow <- struct{}{}:
	default:
	}
}

// Close 结束 resolver 生命周期，watch 会在收到 ctx.Done() 后退出。
func (r *grpcResolver) Close() { r.cancel() }

// watch 持续把 Consul 中的最新实例同步给 grpc.ClientConn。
// 刷新由定时器或 grpc-go 的 ResolveNow 请求触发。
func (r *grpcResolver) watch() {
	ticker := time.NewTicker(grpcRefreshPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return // ClientConn 已关闭，不再继续发现服务。
		case <-ticker.C:
			// 到达定时刷新周期。
		case <-r.resolveNow:
			// grpc-go 要求立即刷新地址。
		}
		if err := r.resolve(); err != nil {
			// 本次发现失败时通知 grpc-go；上一次成功的地址仍可继续使用。
			r.clientConn.ReportError(err)
		}
	}
}

// resolve 执行一次完整的服务发现：查询 Consul、转换地址、通知 grpc-go。
func (r *grpcResolver) resolve() error {
	// 单次请求设置独立超时，同时父级 r.ctx 取消时也会立即结束。
	ctx, cancel := context.WithTimeout(r.ctx, grpcDiscoveryTimeout)
	defer cancel()

	entries, err := r.discoverer.DiscoverGRPCService(ctx, r.serviceName)
	if err != nil {
		return fmt.Errorf("discover gRPC service %q: %w", r.serviceName, err)
	}
	// 把 Consul ServiceEntry 转换成 grpc-go 能识别的 resolver.Address。
	addresses := grpcResolverAddresses(entries)
	if len(addresses) == 0 {
		return fmt.Errorf("gRPC service %q has no healthy instances", r.serviceName)
	}
	// UpdateState 必须提交当前完整地址列表，不是只提交新增地址。
	// grpc-go 会据此新增、复用或移除内部 SubConn，并交给 round_robin 选择。
	if err := r.clientConn.UpdateState(resolver.State{Addresses: addresses}); err != nil {
		return fmt.Errorf("update gRPC resolver state for %q: %w", r.serviceName, err)
	}
	return nil
}

// grpcResolverAddresses 校验、格式化并去重 Consul 实例地址。
func grpcResolverAddresses(entries []*api.ServiceEntry) []resolver.Address {
	// 长度从 0 开始，因为无效实例会被过滤；容量提前按 entries 数量分配以减少扩容。
	addresses := make([]resolver.Address, 0, len(entries))
	// map[string]struct{} 作为 Set 使用，只记录某个 host:port 是否已经出现。
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		// 单个实例无效时直接跳过，不影响其他健康实例。
		if entry == nil || entry.Service == nil || entry.Service.Port <= 0 || entry.Service.Port > 65535 {
			continue
		}
		// 优先使用服务注册地址；为空时回退到 Consul 节点地址。
		host := strings.TrimSpace(entry.Service.Address)
		if host == "" && entry.Node != nil {
			host = strings.TrimSpace(entry.Node.Address)
		}
		if host == "" {
			continue
		}
		// JoinHostPort 会正确生成 IPv4、域名和 IPv6 的 host:port 格式。
		address := net.JoinHostPort(host, strconv.Itoa(entry.Service.Port))
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, resolver.Address{Addr: address})
	}
	return addresses
}
