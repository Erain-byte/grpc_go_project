package consul

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"gateway/internal/config"
	"gateway/pkg/apperror"

	"github.com/hashicorp/consul/api"
)

func TestRegisterHTTPUsesHTTPTagAndHealthCheck(t *testing.T) {
	var got api.AgentServiceRegistration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode registration: %v", err)
		}
	}))
	defer server.Close()

	registry := newTestRegistry(t, server.URL)
	registry.config = config.ConsulConfig{Scheme: "http", CheckHost: "host.docker.internal", CheckInterval: "10s", CheckTimeout: "5s"}
	if err := registry.RegisterHTTP("gateway", "127.0.0.1", 8080, validServiceConfig()); err != nil {
		t.Fatalf("RegisterHTTP() error = %v", err)
	}
	if got.ID != "gateway-http-127.0.0.1-8080" || got.Name != "gateway-http" || got.Port != 8080 || !slices.Contains(got.Tags, ProtocolHTTP) || slices.Contains(got.Tags, ProtocolGRPC) {
		t.Fatalf("HTTP registration = %+v", got)
	}
	if got.Check == nil || got.Check.HTTP != "http://host.docker.internal:8080/health" {
		t.Fatalf("HTTP health check = %+v", got.Check)
	}
}

func TestRegisterGRPCUsesGRPCTagAndHealthCheck(t *testing.T) {
	var got api.AgentServiceRegistration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode registration: %v", err)
		}
	}))
	defer server.Close()

	registry := newTestRegistry(t, server.URL)
	registry.config = config.ConsulConfig{CheckHost: "host.docker.internal", CheckInterval: "10s", CheckTimeout: "5s"}
	if err := registry.RegisterGRPC("llm-service", "127.0.0.1", 9080, validServiceConfig()); err != nil {
		t.Fatalf("RegisterGRPC() error = %v", err)
	}
	if got.ID != "llm-service-grpc-127.0.0.1-9080" || got.Name != "llm-service-grpc" || got.Port != 9080 || !slices.Contains(got.Tags, ProtocolGRPC) || slices.Contains(got.Tags, ProtocolHTTP) {
		t.Fatalf("gRPC registration = %+v", got)
	}
	if got.Check == nil || got.Check.GRPC != "host.docker.internal:9080" {
		t.Fatalf("gRPC health check = %+v", got.Check)
	}
}

func TestQueryConsulFiltersByProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tag"); got != ProtocolGRPC {
			t.Errorf("tag query = %q, want %q", got, ProtocolGRPC)
		}
		if got := r.URL.Query().Get("passing"); got != "1" {
			t.Errorf("passing query = %q, want 1", got)
		}
		w.Header().Set("X-Consul-Index", "42")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	entries, index, err := newTestRegistry(t, server.URL).queryGRPCService(context.Background(), "user-service", 0)
	if err != nil {
		t.Fatalf("queryGRPCService() error = %v", err)
	}
	if len(entries) != 0 || index != 42 {
		t.Fatalf("queryGRPCService() = (%d entries, index %d), want (0, 42)", len(entries), index)
	}
}

func TestQueryConsulPropagatesCancellation(t *testing.T) {
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err := newTestRegistry(t, server.URL).queryGRPCService(ctx, "user-service", 1)
	if !errors.Is(err, context.DeadlineExceeded) || apperror.As(err).Code != apperror.CodeTimeout {
		t.Fatalf("queryGRPCService() error = %v, want timeout with deadline cause", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("Consul request did not observe cancellation")
	}
}

func TestWatchGRPCServiceAdvancesConsulIndex(t *testing.T) {
	var mu sync.Mutex
	indexes := make([]string, 0, 2)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		indexes = append(indexes, r.URL.Query().Get("index"))
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		if currentRequest == 1 {
			w.Header().Set("X-Consul-Index", "10")
		} else {
			w.Header().Set("X-Consul-Index", "11")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	wantStop := errors.New("stop watch")
	updateCount := 0
	err := newTestRegistry(t, server.URL).WatchGRPCService(
		context.Background(),
		"admin-service-grpc",
		func([]*api.ServiceEntry) error {
			updateCount++
			if updateCount == 2 {
				return wantStop
			}
			return nil
		},
	)
	if !errors.Is(err, wantStop) {
		t.Fatalf("WatchGRPCService() error = %v, want %v", err, wantStop)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(indexes) != 2 {
		t.Fatalf("Consul query indexes = %v, want 2 requests", indexes)
	}
	if indexes[0] != "" || indexes[1] != "10" {
		t.Fatalf("Consul query indexes = %v, want [empty 10]", indexes)
	}
}

func TestWatchGRPCServiceValidatesInput(t *testing.T) {
	registry := &ConsulRegistry{}
	validCallback := func([]*api.ServiceEntry) error { return nil }
	tests := []struct {
		name     string
		ctx      context.Context
		service  string
		callback func([]*api.ServiceEntry) error
	}{
		{name: "nil context", service: "admin-service-grpc", callback: validCallback},
		{name: "empty service", ctx: context.Background(), service: "  ", callback: validCallback},
		{name: "nil callback", ctx: context.Background(), service: "admin-service-grpc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := registry.WatchGRPCService(test.ctx, test.service, test.callback); err == nil {
				t.Fatal("WatchGRPCService() error = nil, want validation error")
			}
		})
	}
}

func validServiceConfig() *config.Config {
	return &config.Config{Name: "gateway", Service: config.ServiceConfig{CTags: []string{"gateway"}}}
}

func newTestRegistry(t *testing.T, address string) *ConsulRegistry {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.Address = address
	client, err := api.NewClient(cfg)
	if err != nil {
		t.Fatalf("api.NewClient() error = %v", err)
	}
	return &ConsulRegistry{client: client}
}
