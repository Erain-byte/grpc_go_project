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
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// GRPC服务器
type GRPCServer struct {
	server   *grpc.Server
	health   *health.Server
	listener net.Listener
	address  string
}

// 构造gRPC服务器
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
	// 注册健康检查服务
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	// 空字符串代表整个 gRPC Server，而不是某一个具体业务服务。
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	forwarder.RegisterAllGRPCServices(grpcServer, svcCtx, clientManager)

	return &GRPCServer{
		server:   grpcServer,
		health:   healthServer,
		listener: listener,
		address:  address,
	}, nil
}

// 启动gRPC服务器
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

// 关闭gRPC服务器
func (s *GRPCServer) Shutdown(ctx context.Context) error {
	// 先告诉 Consul/调用方该实例不再接收新流量，再等待现有 RPC 完成。
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

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
