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

const grpcResolverScheme = "consul"

type GRPCServiceWatcher interface {
	WatchGRPCService(context.Context, string, func([]*api.ServiceEntry) error) error
}

type GRPCResolverBuilder struct{ watcher GRPCServiceWatcher }

type grpcResolver struct {
	watcher       GRPCServiceWatcher
	clientConn    resolver.ClientConn
	serviceName   string
	ctx           context.Context
	cancel        context.CancelFunc
	lastAddresses []resolver.Address
}

var _ GRPCServiceWatcher = (*ConsulRegistry)(nil)
var _ resolver.Builder = (*GRPCResolverBuilder)(nil)
var _ resolver.Resolver = (*grpcResolver)(nil)

func NewGRPCResolverBuilder(watcher GRPCServiceWatcher) *GRPCResolverBuilder {
	return &GRPCResolverBuilder{watcher: watcher}
}

func (b *GRPCResolverBuilder) Scheme() string { return grpcResolverScheme }

// Build 由grpc.NewClient根据consul:///service-name目标自动调用。
func (b *GRPCResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	if b.watcher == nil {
		return nil, fmt.Errorf("Consul service watcher is nil")
	}
	serviceName := strings.TrimSpace(target.Endpoint())
	if serviceName == "" {
		return nil, fmt.Errorf("Consul service name is empty")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &grpcResolver{watcher: b.watcher, clientConn: cc, serviceName: serviceName, ctx: ctx, cancel: cancel}
	go r.watch()
	return r, nil
}

func (r *grpcResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (r *grpcResolver) Close()                                { r.cancel() }

func (r *grpcResolver) watch() {
	delay := 500 * time.Millisecond
	for {
		updated := false
		err := r.watcher.WatchGRPCService(r.ctx, r.serviceName, func(entries []*api.ServiceEntry) error {
			if err := r.updateAddresses(entries); err != nil {
				return err
			}
			updated = true
			return nil
		})
		if r.ctx.Err() != nil {
			return
		}
		if updated {
			delay = 500 * time.Millisecond
		}
		r.clientConn.ReportError(fmt.Errorf("watch gRPC service %q: %w", r.serviceName, err))
		if !waitRetry(r.ctx, delay) {
			return
		}
		delay *= 2
		if delay > 15*time.Second {
			delay = 15 * time.Second
		}
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

func (r *grpcResolver) updateAddresses(entries []*api.ServiceEntry) error {
	addresses := grpcResolverAddresses(entries)
	if resolverAddressesEqual(r.lastAddresses, addresses) {
		return nil
	}
	if err := r.clientConn.UpdateState(resolver.State{Addresses: addresses}); err != nil {
		r.lastAddresses = nil
		return fmt.Errorf("update gRPC resolver state for %q: %w", r.serviceName, err)
	}
	r.lastAddresses = slices.Clone(addresses)
	return nil
}

func grpcResolverAddresses(entries []*api.ServiceEntry) []resolver.Address {
	addresses := make([]resolver.Address, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.Service == nil || entry.Service.Port < 1 || entry.Service.Port > 65535 {
			continue
		}
		host := strings.TrimSpace(entry.Service.Address)
		if host == "" && entry.Node != nil {
			host = strings.TrimSpace(entry.Node.Address)
		}
		if host == "" {
			continue
		}
		address := net.JoinHostPort(host, strconv.Itoa(entry.Service.Port))
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, resolver.Address{Addr: address})
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Addr < addresses[j].Addr })
	return addresses
}

func resolverAddressesEqual(left, right []resolver.Address) bool {
	return slices.EqualFunc(left, right, func(a, b resolver.Address) bool { return a.Addr == b.Addr })
}
