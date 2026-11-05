package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/config"
)

func TestNewPprofServerDisabled(t *testing.T) {
	server, err := NewPprofServer(config.PprofConfig{})
	if err != nil || server != nil {
		t.Fatalf("server/error = %#v/%v, want nil/nil", server, err)
	}
}

func TestNewPprofServerRejectsRemoteHostByDefault(t *testing.T) {
	_, err := NewPprofServer(config.PprofConfig{Enabled: true, Host: "0.0.0.0", Port: 6060})
	if err == nil {
		t.Fatal("expected non-loopback host to be rejected")
	}
}

func TestNewPprofServerAllowsExplicitRemoteHost(t *testing.T) {
	server, err := NewPprofServer(config.PprofConfig{
		Enabled:     true,
		Host:        "0.0.0.0",
		Port:        6060,
		AllowRemote: true,
	})
	if err != nil || server == nil {
		t.Fatalf("server/error = %#v/%v", server, err)
	}
}

func TestPprofMuxExposesOnlyDebugRoutes(t *testing.T) {
	mux := newPprofMux()

	index := httptest.NewRecorder()
	mux.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("pprof index status = %d, want 200", index.Code)
	}

	heap := httptest.NewRecorder()
	mux.ServeHTTP(heap, httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil))
	if heap.Code != http.StatusOK {
		t.Fatalf("heap status = %d, want 200", heap.Code)
	}

	unknown := httptest.NewRecorder()
	mux.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/health", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unregistered route status = %d, want 404", unknown.Code)
	}
}
