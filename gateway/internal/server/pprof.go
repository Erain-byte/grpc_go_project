package server

import (
	"admin/pkg/apperorr"
	"context"
	"errors"
	"fmt"
	"gateway/internal/config"
	"net"
	"net/http"
	_ "net/http/pprof"
)

type PprofServer struct {
	server *http.Server
}

func NewPprofServer(cfg config.PprofConfig) (*PprofServer, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, apperorr.InvalidArgument("pprof port must be between 1 and 65535")
	}
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return &PprofServer{
		server: &http.Server{
			Addr: net.JoinHostPort(
				host,
				fmt.Sprintf("%d", cfg.Port),
			),
			Handler: http.DefaultServeMux,
		},
	}, nil
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
