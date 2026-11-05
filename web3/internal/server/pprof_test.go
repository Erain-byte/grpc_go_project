package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"web3/internal/config"
)

func TestNewPprofServerDisabled(t *testing.T) {
	server, err := NewPprofServer(config.PprofConfig{})
	if err != nil || server != nil {
		t.Fatalf("server/error = %#v/%v, want nil/nil", server, err)
	}
}

func TestNewPprofServerRejectsRemoteHostByDefault(t *testing.T) {
	_, err := NewPprofServer(config.PprofConfig{Enabled: true, Host: "0.0.0.0", Port: 6063})
	if err == nil {
		t.Fatal("expected non-loopback host to be rejected")
	}
}

func TestPprofMuxExposesOnlyDebugRoutes(t *testing.T) {
	mux := newPprofMux()
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/goroutine"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, response.Code)
		}
	}
	unknown := httptest.NewRecorder()
	mux.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/health", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unregistered route status = %d, want 404", unknown.Code)
	}
}
