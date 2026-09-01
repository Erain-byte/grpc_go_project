package consul

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

type watchUpdate struct {
	entries []*api.ServiceEntry
	err     error
}

// fakeGRPCWatcher由测试主动发送服务变化，模拟Consul Blocking Query返回。
type fakeGRPCWatcher struct {
	mu      sync.Mutex
	names   []string
	updates chan watchUpdate
	started chan struct{}
	stopped chan struct{}
	start   sync.Once
	stop    sync.Once
}

func newFakeGRPCWatcher() *fakeGRPCWatcher {
	return &fakeGRPCWatcher{
		updates: make(chan watchUpdate, 8),
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (f *fakeGRPCWatcher) WatchGRPCService(
	ctx context.Context,
	name string,
	onUpdate func([]*api.ServiceEntry) error,
) error {
	f.mu.Lock()
	f.names = append(f.names, name)
	f.mu.Unlock()
	f.start.Do(func() { close(f.started) })

	for {
		select {
		case <-ctx.Done():
			f.stop.Do(func() { close(f.stopped) })
			return ctx.Err()
		case update := <-f.updates:
			if update.err != nil {
				return update.err
			}
			if err := onUpdate(update.entries); err != nil {
				return err
			}
		}
	}
}

func serviceEntry(address string, port int) *api.ServiceEntry {
	return &api.ServiceEntry{Service: &api.AgentService{Address: address, Port: port}}
}

func TestGRPCResolverAddresses(t *testing.T) {
	entries := []*api.ServiceEntry{
		nil,
		serviceEntry("127.0.0.1", 50051),
		serviceEntry("127.0.0.1", 50051),
		{Node: &api.Node{Address: "2001:db8::1"}, Service: &api.AgentService{Port: 50052}},
		serviceEntry("127.0.0.2", 0),
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
	states    chan resolver.State
	errors    chan error
	updateErr error
}

func (f *fakeResolverClientConn) UpdateState(state resolver.State) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.states <- state
	return nil
}

func (f *fakeResolverClientConn) ReportError(err error) { f.errors <- err }

func (f *fakeResolverClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}

func newFakeResolverClientConn() *fakeResolverClientConn {
	return &fakeResolverClientConn{
		states: make(chan resolver.State, 8),
		errors: make(chan error, 8),
	}
}

func resolverTarget(serviceName string) resolver.Target {
	target := resolver.Target{}
	target.URL.Scheme = grpcResolverScheme
	target.URL.Path = "/" + serviceName
	return target
}

func receiveResolverState(t *testing.T, states <-chan resolver.State) resolver.State {
	t.Helper()
	select {
	case state := <-states:
		return state
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resolver state")
		return resolver.State{}
	}
}

func TestGRPCResolverPublishesInitialState(t *testing.T) {
	watcher := newFakeGRPCWatcher()
	watcher.updates <- watchUpdate{entries: []*api.ServiceEntry{
		serviceEntry("127.0.0.1", 50051),
		serviceEntry("127.0.0.2", 50052),
	}}
	cc := newFakeResolverClientConn()
	built, err := NewGRPCResolverBuilder(watcher).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(built.Close)

	state := receiveResolverState(t, cc.states)
	if len(state.Addresses) != 2 {
		t.Fatalf("resolver addresses = %+v, want 2 addresses", state.Addresses)
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if len(watcher.names) != 1 || watcher.names[0] != "admin-service-grpc" {
		t.Fatalf("watch names = %v", watcher.names)
	}
}

func TestGRPCResolverPublishesEmptyState(t *testing.T) {
	watcher := newFakeGRPCWatcher()
	cc := newFakeResolverClientConn()
	built, err := NewGRPCResolverBuilder(watcher).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(built.Close)

	watcher.updates <- watchUpdate{entries: []*api.ServiceEntry{serviceEntry("127.0.0.1", 50051)}}
	if got := len(receiveResolverState(t, cc.states).Addresses); got != 1 {
		t.Fatalf("initial address count = %d, want 1", got)
	}
	watcher.updates <- watchUpdate{entries: []*api.ServiceEntry{}}
	if got := len(receiveResolverState(t, cc.states).Addresses); got != 0 {
		t.Fatalf("empty address count = %d, want 0", got)
	}
}

func TestGRPCResolverSkipsEquivalentAddressState(t *testing.T) {
	watcher := newFakeGRPCWatcher()
	cc := newFakeResolverClientConn()
	built, err := NewGRPCResolverBuilder(watcher).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(built.Close)

	watcher.updates <- watchUpdate{entries: []*api.ServiceEntry{
		serviceEntry("127.0.0.2", 50052), serviceEntry("127.0.0.1", 50051),
	}}
	_ = receiveResolverState(t, cc.states)
	watcher.updates <- watchUpdate{entries: []*api.ServiceEntry{
		serviceEntry("127.0.0.1", 50051), serviceEntry("127.0.0.2", 50052),
	}}
	select {
	case state := <-cc.states:
		t.Fatalf("unexpected duplicate resolver state: %+v", state)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestGRPCResolverReportsWatchError(t *testing.T) {
	watcher := newFakeGRPCWatcher()
	cc := newFakeResolverClientConn()
	built, err := NewGRPCResolverBuilder(watcher).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	t.Cleanup(built.Close)

	wantErr := errors.New("Consul unavailable")
	watcher.updates <- watchUpdate{err: wantErr}
	select {
	case gotErr := <-cc.errors:
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("reported error = %v, want wrapped %v", gotErr, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resolver error")
	}
}

func TestGRPCResolverDoesNotRememberFailedState(t *testing.T) {
	wantErr := errors.New("update state failed")
	cc := newFakeResolverClientConn()
	cc.updateErr = wantErr
	r := &grpcResolver{
		clientConn:  cc,
		serviceName: "admin-service-grpc",
	}

	err := r.updateAddresses([]*api.ServiceEntry{serviceEntry("127.0.0.1", 50051)})
	if !errors.Is(err, wantErr) {
		t.Fatalf("updateAddresses() error = %v, want wrapped %v", err, wantErr)
	}
	if len(r.lastAddresses) != 0 {
		t.Fatalf("lastAddresses = %+v after failed UpdateState", r.lastAddresses)
	}
}

func TestGRPCResolverCloseStopsWatch(t *testing.T) {
	watcher := newFakeGRPCWatcher()
	cc := newFakeResolverClientConn()
	built, err := NewGRPCResolverBuilder(watcher).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	select {
	case <-watcher.started:
	case <-time.After(time.Second):
		t.Fatal("watch did not start")
	}
	built.Close()
	select {
	case <-watcher.stopped:
	case <-time.After(time.Second):
		t.Fatal("watch did not stop after resolver Close")
	}
}

func TestGRPCResolverRejectsInvalidBuilderInput(t *testing.T) {
	cc := newFakeResolverClientConn()
	if _, err := NewGRPCResolverBuilder(nil).Build(
		resolverTarget("admin-service-grpc"), cc, resolver.BuildOptions{},
	); err == nil {
		t.Fatal("Build() error = nil for nil watcher")
	}
	if _, err := NewGRPCResolverBuilder(newFakeGRPCWatcher()).Build(
		resolverTarget(""), cc, resolver.BuildOptions{},
	); err == nil {
		t.Fatal("Build() error = nil for empty service name")
	}
}
