package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"time"

	"user/internal/config"
	"user/pkg/apperror"
)

type PprofServer struct {
	server *http.Server
}

func NewPprofServer(cfg config.PprofConfig) (*PprofServer, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, apperror.InvalidArgument("pprof port must be between 1 and 65535")
	}
	if cfg.BlockProfileRate < 0 || cfg.MutexProfileFraction < 0 {
		return nil, apperror.InvalidArgument("pprof block and mutex sampling rates cannot be negative")
	}
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	if !cfg.AllowRemote && !isLoopbackHost(host) {
		return nil, apperror.InvalidArgument("pprof must use a loopback host unless allow_remote is enabled")
	}
	runtime.SetBlockProfileRate(cfg.BlockProfileRate)
	runtime.SetMutexProfileFraction(cfg.MutexProfileFraction)
	return &PprofServer{server: &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(cfg.Port)),
		Handler:           newPprofMux(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func newPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}
	return mux
}

func (s *PprofServer) Run() error {
	if s == nil || s.server == nil {
		return nil
	}
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *PprofServer) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
