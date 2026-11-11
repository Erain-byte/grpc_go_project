package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"gateway/internal/config"
	"gateway/internal/forwarder"
	grpcclient "gateway/internal/grpc"
	"gateway/internal/logger"
	"gateway/internal/middleware"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	serverOptions := []grpc.ServerOption{
		grpc.StatsHandler(middleware.NewServerStatsHandler()),
	}
	if svcCtx.Config.GRPCServer.UseTLS {
		transportCredentials, credentialErr := createServerCredentials(svcCtx.Config.GRPCServer)
		if credentialErr != nil {
			_ = listener.Close()
			return nil, credentialErr
		}
		serverOptions = append(serverOptions, grpc.Creds(transportCredentials))
	}
	grpcServer := grpc.NewServer(serverOptions...)
	// 注册健康检查服务
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	// 空字符串代表整个 gRPC Server，而不是某一个具体业务服务。
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	forwarder.RegisterAllGRPCServices(grpcServer, clientManager)

	return &GRPCServer{
		server:   grpcServer,
		health:   healthServer,
		listener: listener,
		address:  address,
	}, nil
}

func createServerCredentials(cfg config.GRPCServerConfig) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(strings.TrimSpace(cfg.CertFile), strings.TrimSpace(cfg.KeyFile))
	if err != nil {
		return nil, fmt.Errorf("load Gateway gRPC TLS certificate: %w", err)
	}
	if len(certificate.Certificate) == 0 {
		return nil, errors.New("Gateway gRPC TLS certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse Gateway gRPC TLS certificate: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, fmt.Errorf("Gateway gRPC TLS certificate is outside its validity period: %s - %s", leaf.NotBefore, leaf.NotAfter)
	}
	serverName := strings.TrimSpace(cfg.TLSServerName)
	if serverName == "" {
		return nil, errors.New("Gateway gRPC TLS server name is empty")
	}
	// 启动时保证 Consul 服务名与本地服务端证书公共 SAN 一致。
	if err := leaf.VerifyHostname(serverName); err != nil {
		return nil, fmt.Errorf("Gateway gRPC TLS certificate SAN does not match service name %q: %w", serverName, err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}
	if clientCAFile := strings.TrimSpace(cfg.ClientCAFile); clientCAFile != "" {
		pemData, err := os.ReadFile(clientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read Gateway gRPC client CA: %w", err)
		}
		clientCAs := x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(pemData) {
			return nil, errors.New("Gateway gRPC client CA contains no valid certificate")
		}
		tlsConfig.ClientCAs = clientCAs
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(tlsConfig), nil
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
