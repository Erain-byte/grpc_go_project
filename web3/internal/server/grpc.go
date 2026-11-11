// Package server 负责 Web3 gRPC 服务的监听、注册和优雅关闭。
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"web3/internal/config"
	"web3/internal/handler"
	"web3/internal/middleware"
	"web3/internal/svc"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

type GRPCServer struct {
	server      *grpc.Server
	health      *health.Server
	listener    net.Listener
	healthName  string
	healthReady bool
}

func NewGRPCServer(cfg *config.Config, svcCtx *svc.ServiceContext) (*GRPCServer, error) {
	if cfg == nil || svcCtx == nil || svcCtx.Wallets == nil || svcCtx.Approvals == nil {
		return nil, errors.New("Web3 gRPC server dependencies are incomplete")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.GRPCPort)))
	if err != nil {
		return nil, fmt.Errorf("listen for Web3 gRPC: %w", err)
	}
	closeListener := true
	defer func() {
		if closeListener {
			_ = listener.Close()
		}
	}()

	authInterceptor, err := middleware.NewAuthInterceptor(cfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("create Web3 auth interceptor: %w", err)
	}
	rateLimitInterceptor := middleware.NewRateLimit(
		cfg.RateLimit.Enabled,
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.Burst,
		cfg.RateLimit.MaxUnaryConcurrency,
	)
	accessLogInterceptor := middleware.NewAccessLog(svcCtx.Logger)
	options := []grpc.ServerOption{
		// 从 gRPC metadata 提取 Gateway/User 注入的 traceparent，并创建 Web3 服务端 Span。
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			middleware.RequestID(),
			accessLogInterceptor.Unary(),
			middleware.Recovery(svcCtx.Logger),
			middleware.ErrorHandler(svcCtx.Logger),
			rateLimitInterceptor.Unary(),
			authInterceptor.Unary(),
		),
	}
	if cfg.GRPC.MaxRecvMessageSize > 0 {
		options = append(options, grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMessageSize))
	}
	if cfg.GRPC.MaxSendMessageSize > 0 {
		options = append(options, grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMessageSize))
	}
	if cfg.GRPC.UseTLS {
		transportCredentials, credentialErr := serverTransportCredentials(cfg.GRPC)
		if credentialErr != nil {
			return nil, credentialErr
		}
		options = append(options, grpc.Creds(transportCredentials))
	}

	grpcServer := grpc.NewServer(options...)
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	web3v1.RegisterWeb3ServiceServer(grpcServer, handler.NewWeb3Handler(svcCtx))

	closeListener = false
	return &GRPCServer{
		server:      grpcServer,
		health:      healthServer,
		listener:    listener,
		healthName:  strings.TrimSpace(cfg.GRPC.Health.Service),
		healthReady: cfg.GRPC.Health.Enabled,
	}, nil
}

func (s *GRPCServer) Start() error {
	if s.healthReady {
		s.health.SetServingStatus(s.healthName, healthv1.HealthCheckResponse_SERVING)
	}
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("serve Web3 gRPC: %w", err)
	}
	return nil
}

func (s *GRPCServer) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	if s.healthReady {
		s.health.SetServingStatus(s.healthName, healthv1.HealthCheckResponse_NOT_SERVING)
	}
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
		return ctx.Err()
	}
}

func (s *GRPCServer) Address() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func serverTransportCredentials(cfg config.GRPCConfig) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(strings.TrimSpace(cfg.CertFile), strings.TrimSpace(cfg.KeyFile))
	if err != nil {
		return nil, fmt.Errorf("load Web3 gRPC TLS certificate: %w", err)
	}
	// Consul 服务名必须与证书公共 SAN 一致；不一致时在启动阶段直接失败。
	if err := verifyCertificateServerName(certificate, cfg.TLSServerName); err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}
	if clientCAFile := strings.TrimSpace(cfg.ClientCAFile); clientCAFile != "" {
		pemData, err := os.ReadFile(clientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read Web3 gRPC client CA: %w", err)
		}
		clientCAs := x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(pemData) {
			return nil, errors.New("Web3 gRPC client CA contains no valid certificate")
		}
		tlsConfig.ClientCAs = clientCAs
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(tlsConfig), nil
}

func verifyCertificateServerName(certificate tls.Certificate, serverName string) error {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return errors.New("Web3 gRPC TLS server name is empty")
	}
	if len(certificate.Certificate) == 0 {
		return errors.New("Web3 gRPC TLS certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse Web3 gRPC TLS certificate: %w", err)
	}
	if err := leaf.VerifyHostname(serverName); err != nil {
		return fmt.Errorf("Web3 gRPC TLS certificate SAN does not match service name %q: %w", serverName, err)
	}
	return nil
}
