package server

import (
	"admin/internal/config"
	"admin/internal/handler"
	"admin/internal/middleware"
	"admin/internal/svc"
	"context"
	"errors"
	"net"
	"strconv"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCServer struct {
	// todo
	server   *grpc.Server
	health   *health.Server
	listener net.Listener
}

func NewGRPCServer(
	cfg config.Config,
	svc *svc.ServiceContext,
) (*GRPCServer, error) {

	//组装addreses
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.GRPCPort))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	serverOptions := make([]grpc.ServerOption, 0, 2) //初始化grpc服务选项
	//链路追踪
	serverOptions = append(
		serverOptions,
		grpc.StatsHandler(
			otelgrpc.NewServerHandler(),
		),
		grpc.ChainUnaryInterceptor(
			middleware.RequestID(),
			middleware.ErrorHandler(),
		),
	)
	//服务端TLS
	if cfg.GRPC.UseTLS {
		creds, err := credentials.NewServerTLSFromFile(
			cfg.GRPC.CertFile,
			cfg.GRPC.KeyFile,
		)
		if err != nil {
			_ = listener.Close() //关闭监听
			return nil, err
		}
		serverOptions = append(
			serverOptions,
			grpc.Creds(creds),
		)
	}
	//初始化grpc服务
	grpacServer := grpc.NewServer(serverOptions...)
	//初始化健康检查
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpacServer, healthServer) //注册健康检查

	adminHandler := handler.NewAdminHandler(svc) //初始化handler

	adminv1.RegisterAdminServiceServer(
		grpacServer,
		adminHandler,
	)

	return &GRPCServer{
		server:   grpacServer,
		health:   healthServer,
		listener: listener,
	}, nil

}

// Start 启动grpc服务
func (s *GRPCServer) Start() error {
	//return s.server.Serve(s.listener)
	s.health.SetServingStatus( //设置健康检查状态
		"",
		healthpb.HealthCheckResponse_SERVING,
	)
	err := s.server.Serve(s.listener)
	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// 优雅关闭
func (s *GRPCServer) Stop(ctx context.Context) error {
	//设置健康检查状态
	s.health.SetServingStatus(
		"",
		healthpb.HealthCheckResponse_NOT_SERVING,
	)
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop() //优雅关闭
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		<-done
		return ctx.Err()
	}

}
