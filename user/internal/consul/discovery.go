package consul

import (
	"context"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc/resolver"
)

//当前脚本实现consul 的 discovery resolver功能

const grpcResolverScheme = "consul"

var _ GRPCServiceWatcher = (*ConsulRegistry)(nil)

var _ resolver.Builder = (*GRPCResolverBuilder)(nil)
var _ resolver.Resolver = (*grpcResolver)(nil)

// GRPCServiceWatcher 定义 resolver 所需的最小 Consul 监听能力。
type GRPCServiceWatcher interface {
	WatchGRPCService(
		ctx context.Context,
		serviceName string,
		onUpdate func([]*api.ServiceEntry) error,
	) error
}

// GRPCResolverBuilder 是 Consul 服务发现与 grpc-go resolver 之间的适配器。
type GRPCResolverBuilder struct {
	watcher GRPCServiceWatcher
}

// grpcResolver 负责一个逻辑 gRPC 服务的持续地址解析。
type grpcResolver struct {
	watcher     GRPCServiceWatcher
	clientConn  resolver.ClientConn
	serviceName string
	ctx         context.Context
	cancel      context.CancelFunc
	//最后一次返回的地址列表
	lastAddresses []resolver.Address
}

// 构造函数
func NewGRPCResolverBuilder(watcher GRPCServiceWatcher) *GRPCResolverBuilder {
	return &GRPCResolverBuilder{watcher: watcher}
}

// Scheme 返回 resolver 的协议名，对应 target 中的 consul:// 前缀。
func (b *GRPCResolverBuilder) Scheme() string {
	return grpcResolverScheme
}

// Build 根据逻辑服务名创建 resolver。
func (b *GRPCResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	if b.watcher == nil {
		return nil, fmt.Errorf("consul: no watcher provided")
	}
	//获取服务名
	serviceName := strings.TrimSpace(target.Endpoint())
	if serviceName == "" {
		return nil, fmt.Errorf("consul: no service name provided")
	}
	//设置ctx
	ctx, cancel := context.WithCancel(context.Background())
	r := &grpcResolver{
		watcher:     b.watcher,
		clientConn:  cc,
		serviceName: serviceName,
		ctx:         ctx,
		cancel:      cancel,
	}
	//启动监听
	go r.watch()
	return r, nil
}

// ResolveNow 接收 grpc-go 的立即重新解析请求。
func (r *grpcResolver) ResolveNow(o resolver.ResolveNowOptions) {
}

// close 关闭 resolver。
func (r *grpcResolver) Close() {
	r.cancel()
}

// watcher 实现了 resolver.Resolver 接口持续把 Consul 中的最新实例同步给 grpc.ClientConn。
func (r *grpcResolver) watch() {
	//重试时间
	retryDelay := 500 * time.Millisecond
	for {
		update := false
		err := r.watcher.WatchGRPCService(
			r.ctx, r.serviceName,
			func(entries []*api.ServiceEntry) error {
				if err := r.updateAddresses(entries); err != nil {
					return err
				}
				update = true
				return nil
			})
		if r.ctx.Err() != nil {
			return
		}
		if update {
			retryDelay = 500 * time.Millisecond
		}
		r.clientConn.ReportError(
			fmt.Errorf("watch gRPC service %q: %w", r.serviceName, err),
		)
		//重试
		if !waitRetry(r.ctx, retryDelay) {
			return
		}
		retryDelay = retryDelay * 2
		if retryDelay > 15*time.Second {
			retryDelay = 15 * time.Second
		}
	}

}

// 更新updateAddresses
func (r *grpcResolver) updateAddresses(entries []*api.ServiceEntry) error {
	//获取地址
	addres := grpcResolverAddresses(entries)
	if resolverAddresEqual(r.lastAddresses, addres) {
		return nil
	}
	//更新地址
	state := resolver.State{Addresses: addres}
	if err := r.clientConn.UpdateState(state); err != nil {
		//清楚缓存
		r.lastAddresses = nil
		return fmt.Errorf("update gRPC resolver state for %q: %w", r.serviceName, err)
	}
	//更新缓存
	r.lastAddresses = slices.Clone(addres)
	return nil
}

// grpcResolverAddresses 校验、格式化并去重 Consul 实例地址
func grpcResolverAddresses(entries []*api.ServiceEntry) []resolver.Address {
	// 长度从 0 开始，因为无效实例会被过滤；容量提前按 entries 数量分配以减少扩容。
	addresses := make([]resolver.Address, 0, len(entries))
	// map[string]struct{} 作为 Set 使用，只记录某个 host:port 是否已经出现
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.Service == nil || entry.Service.Port <= 0 || entry.Service.Port > 65535 {
			continue
		}
		// 优先使用服务注册地址；为空时回退到 Consul 节点地址。
		host := strings.TrimSpace(entry.Service.Address)
		if host == "" || entry.Node != nil {
			host = strings.TrimSpace(entry.Node.Address)
		}
		if host == "" {
			continue
		}
		// JoinHostPort 会正确生成 IPv4、域名和 IPv6 的 host:port 格式。
		add := net.JoinHostPort(host, strconv.Itoa(entry.Service.Port))
		if _, ok := seen[add]; ok {
			continue
		}
		//占位
		seen[add] = struct{}{}
		addresses = append(addresses, resolver.Address{Addr: add})
	}
	// 排序使相同实例集合始终得到相同顺序，避免无意义地更新 ClientConn。
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].Addr < addresses[j].Addr
	})
	return addresses
}

// 比较新旧地址列表，并通知给 grpc.ClientConn
func resolverAddresEqual(left, right []resolver.Address) bool {
	return slices.EqualFunc(
		left,
		right,
		func(a, b resolver.Address) bool {
			return a.Addr == b.Addr
		},
	)
}
