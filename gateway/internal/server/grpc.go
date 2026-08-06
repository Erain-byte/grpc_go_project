package server

import (
	"context"
	"errors"
	"net"
	"net/http"

	"gateway/internal/forwarder"
	grpcclient "gateway/internal/grpc"
	"gateway/internal/logger"
	"gateway/internal/middleware"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	"google.golang.org/grpc"
)

// GRPCServer owns the Gateway gRPC server and its TCP listener.
type GRPCServer struct {
	server   *grpc.Server
	listener net.Listener
	address  string
}

// NewGRPCServer creates the listener, configures middleware and registers all
// Gateway gRPC services. It does not start serving until Start is called.
func NewGRPCServer(
	address string,
	svcCtx *svc.ServiceContext,
	clientManager *grpcclient.ClientManager,
) (*GRPCServer, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"failed to listen for gRPC on "+address,
			http.StatusServiceUnavailable,
		)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(middleware.NewServerStatsHandler()),
	)
	forwarder.RegisterAllGRPCServices(grpcServer, svcCtx, clientManager)

	return &GRPCServer{
		server:   grpcServer,
		listener: listener,
		address:  address,
	}, nil
}

// Start serves gRPC requests until Shutdown is called or serving fails.
func (s *GRPCServer) Start() error {
	logger.SugaredLogger.Infof("Gateway gRPC Server starting at %s", s.address)
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"gRPC server stopped unexpectedly on "+s.address,
			http.StatusServiceUnavailable,
		)
	}
	return nil
}

// Shutdown waits for active RPCs and streams. When ctx expires, Stop forces
// the remaining connections to close.
func (s *GRPCServer) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		<-done
		return apperror.Wrap(
			ctx.Err(),
			apperror.CodeTimeout,
			"gRPC graceful shutdown timed out",
			http.StatusGatewayTimeout,
		)
	}
}
