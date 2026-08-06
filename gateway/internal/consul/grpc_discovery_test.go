package consul

import (
	"context"
	"sync"
	"testing"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

type fakeGRPCDiscoverer struct {
	mu      sync.Mutex
	entries []*api.ServiceEntry
	err     error
	names   []string
}

func (f *fakeGRPCDiscoverer) DiscoverGRPCService(_ context.Context, name string) ([]*api.ServiceEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names = append(f.names, name)
	return f.entries, f.err
}

func TestGRPCResolverAddresses(t *testing.T) {
	entries := []*api.ServiceEntry{
		nil,
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 50051}},
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 50051}},
		{Node: &api.Node{Address: "2001:db8::1"}, Service: &api.AgentService{Port: 50052}},
		{Service: &api.AgentService{Address: "127.0.0.2", Port: 0}},
	}
	got := grpcResolverAddresses(entries)
	if len(got) != 2 {
		t.Fatalf("address count = %d, want 2: %+v", len(got), got)
	}
	if got[0].Addr != "127.0.0.1:50051" {
		t.Fatalf("first address = %q", got[0].Addr)
	}
	if got[1].Addr != "[2001:db8::1]:50052" {
		t.Fatalf("IPv6 address = %q", got[1].Addr)
	}
}

type fakeResolverClientConn struct {
	resolver.ClientConn
	states chan resolver.State
	errors chan error
}

func (f *fakeResolverClientConn) UpdateState(state resolver.State) error {
	f.states <- state
	return nil
}

func (f *fakeResolverClientConn) ReportError(err error) { f.errors <- err }

func (f *fakeResolverClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}

func TestGRPCResolverPublishesInitialState(t *testing.T) {
	discoverer := &fakeGRPCDiscoverer{entries: []*api.ServiceEntry{
		{Service: &api.AgentService{Address: "127.0.0.1", Port: 50051}},
		{Service: &api.AgentService{Address: "127.0.0.2", Port: 50052}},
	}}
	cc := &fakeResolverClientConn{
		states: make(chan resolver.State, 1),
		errors: make(chan error, 1),
	}
	builder := NewGRPCResolverBuilder(discoverer)
	target := resolver.Target{}
	target.URL.Scheme = grpcResolverScheme
	target.URL.Path = "/admin-service-grpc"

	built, err := builder.Build(target, cc, resolver.BuildOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(built.Close)

	select {
	case state := <-cc.states:
		if len(state.Addresses) != 2 {
			t.Fatalf("resolver addresses = %+v", state.Addresses)
		}
	default:
		t.Fatal("resolver did not publish initial state")
	}

	discoverer.mu.Lock()
	defer discoverer.mu.Unlock()
	if len(discoverer.names) != 1 || discoverer.names[0] != "admin-service-grpc" {
		t.Fatalf("discovery names = %v", discoverer.names)
	}
}

func TestGRPCResolverRejectsNoInstances(t *testing.T) {
	builder := NewGRPCResolverBuilder(&fakeGRPCDiscoverer{})
	cc := &fakeResolverClientConn{
		states: make(chan resolver.State, 1),
		errors: make(chan error, 1),
	}
	target := resolver.Target{}
	target.URL.Path = "/admin-service-grpc"
	if _, err := builder.Build(target, cc, resolver.BuildOptions{}); err == nil {
		t.Fatal("Build() error = nil, want no instances error")
	}
}

func TestGRPCResolverHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &grpcResolver{
		discoverer:  &fakeGRPCDiscoverer{},
		ctx:         ctx,
		serviceName: "admin-service-grpc",
	}
	if err := r.resolve(); err == nil {
		t.Fatal("resolve() error = nil for canceled context")
	}
}
