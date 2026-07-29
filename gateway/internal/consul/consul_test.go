package consul

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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
	registry.config = config.ConsulConfig{Scheme: "http", CheckInterval: "10s", CheckTimeout: "5s"}
	if err := registry.RegisterHTTP("gateway", "127.0.0.1", 8080, validServiceConfig()); err != nil {
		t.Fatalf("RegisterHTTP() error = %v", err)
	}
	if got.ID != "gateway-http" || got.Port != 8080 || !slices.Contains(got.Tags, ProtocolHTTP) || slices.Contains(got.Tags, ProtocolGRPC) {
		t.Fatalf("HTTP registration = %+v", got)
	}
	if got.Check == nil || got.Check.HTTP != "http://127.0.0.1:8080/health" {
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
	registry.config = config.ConsulConfig{CheckInterval: "10s", CheckTimeout: "5s"}
	if err := registry.RegisterGRPC("llm-service", "127.0.0.1", 9080, validServiceConfig()); err != nil {
		t.Fatalf("RegisterGRPC() error = %v", err)
	}
	if got.ID != "llm-service-grpc" || got.Port != 9080 || !slices.Contains(got.Tags, ProtocolGRPC) || slices.Contains(got.Tags, ProtocolHTTP) {
		t.Fatalf("gRPC registration = %+v", got)
	}
	if got.Check == nil || got.Check.GRPC != "127.0.0.1:9080" {
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

	entries, index, err := newTestRegistry(t, server.URL).queryConsul(context.Background(), "user-service", ProtocolGRPC, 0)
	if err != nil {
		t.Fatalf("queryConsul() error = %v", err)
	}
	if len(entries) != 0 || index != 42 {
		t.Fatalf("queryConsul() = (%d entries, index %d), want (0, 42)", len(entries), index)
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
	_, _, err := newTestRegistry(t, server.URL).queryConsul(ctx, "user-service", ProtocolGRPC, 1)
	if !errors.Is(err, context.DeadlineExceeded) || apperror.As(err).Code != apperror.CodeTimeout {
		t.Fatalf("queryConsul() error = %v, want timeout with deadline cause", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("Consul request did not observe cancellation")
	}
}

func TestQueryConsulValidatesProtocol(t *testing.T) {
	registry := &ConsulRegistry{}
	_, _, err := registry.queryConsul(context.Background(), "user-service", "", 0)
	if err == nil || apperror.As(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("queryConsul() error = %v, want invalid argument", err)
	}
}

func TestCacheSeparatesHTTPAndGRPCServices(t *testing.T) {
	registry := &ConsulRegistry{serviceCache: make(map[string]*serviceCacheEntry)}
	httpKey := serviceCacheKey("user-service", ProtocolHTTP)
	grpcKey := serviceCacheKey("user-service", ProtocolGRPC)
	registry.updateCache(httpKey, nil, 1)
	if _, found := registry.getFromCache(httpKey); !found {
		t.Fatal("HTTP cache entry was not found")
	}
	if _, found := registry.getFromCache(grpcKey); found {
		t.Fatal("gRPC lookup incorrectly used HTTP cache entry")
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
