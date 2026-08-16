package server

import (
	"admin/internal/config"
	"admin/internal/handler"
	"admin/internal/middleware"
	"admin/internal/svc"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const certificateWarnAfter = 24 * time.Hour

type GRPCServer struct {
	// server 负责处理 gRPC 请求，health 对外提供标准健康检查，
	// listener 保存已经绑定的 TCP 监听端口。
	server   *grpc.Server
	health   *health.Server
	listener net.Listener
	certFile string
}

func NewGRPCServer(cfg *config.Config, svcCtx *svc.ServiceContext) (*GRPCServer, error) {
	// JoinHostPort 能正确处理普通 IP、域名以及 IPv6 地址。
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.GRPCPort))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	//初始化JWT
	//tokenVerifier, err := auth.NewJWTVerifier(cfg.Auth.AccessToken)
	/*if err != nil {
		_ = listener.Close()
		return nil, err
	}*/
	//authInterceptor := middleware.NewAuthInterceptor(tokenVerifier)
	options := []grpc.ServerOption{
		// StatsHandler 从 gRPC metadata 提取上游 Trace，并记录服务端 Span。
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		// 一元拦截器按照声明顺序执行：先准备 Request ID，再统一转换业务错误。
		grpc.ChainUnaryInterceptor(
			middleware.RequestID(),
			middleware.ErrorHandler(),
			middleware.NewAuthInterceptor().Unary(),
		),
	}
	if cfg.GRPC.UseTLS {
		creds, remaining, credentialErr := createServerCredentials(cfg.GRPC)
		if credentialErr != nil {
			_ = listener.Close()
			return nil, credentialErr
		}
		if remaining < certificateWarnAfter && svcCtx.Logger != nil {
			svcCtx.Logger.Warn(
				"server TLS certificate will expire soon",
				zap.Duration("remaining", remaining),
			)
		}
		options = append(options, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(options...)
	// 注册标准 gRPC Health 服务后，Consul 才能通过 gRPC 健康检查判断实例状态。
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	adminv1.RegisterAdminServiceServer(grpcServer, handler.NewAdminHandler(svcCtx))

	return &GRPCServer{
		server:   grpcServer,
		health:   healthServer,
		listener: listener,
		certFile: strings.TrimSpace(cfg.GRPC.CertFile),
	}, nil
}

// createServerCredentials 创建服务端 TLS 凭证；配置 ClientCAFile 时启用双向 TLS。
func createServerCredentials(cfg config.GRPCServerConfig) (credentials.TransportCredentials, time.Duration, error) {
	certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, 0, fmt.Errorf("load server TLS certificate: %w", err)
	}
	remaining, err := certificateRemainingValidity(certificate)
	if err != nil {
		return nil, remaining, err
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}
	if clientCAFile := strings.TrimSpace(cfg.ClientCAFile); clientCAFile != "" {
		pemData, readErr := os.ReadFile(clientCAFile)
		if readErr != nil {
			return nil, 0, fmt.Errorf("read gRPC client CA certificate: %w", readErr)
		}
		clientCAs := x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(pemData) {
			return nil, 0, errors.New("gRPC client CA file contains no valid certificate")
		}
		tlsConfig.ClientCAs = clientCAs
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(tlsConfig), remaining, nil
}

func certificateRemainingValidity(certificate tls.Certificate) (time.Duration, error) {
	if len(certificate.Certificate) == 0 {
		return 0, errors.New("server TLS certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return 0, fmt.Errorf("parse server TLS certificate: %w", err)
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return 0, errors.New("server TLS certificate is not valid yet")
	}
	remaining := leaf.NotAfter.Sub(now)
	if remaining <= 0 {
		return remaining, errors.New("server TLS certificate has expired")
	}
	return remaining, nil
}

// MonitorCertificate 定期检查服务端证书；过期记录错误，临期只记录警告。
func (s *GRPCServer) MonitorCertificate(ctx context.Context, interval time.Duration, log *zap.Logger) {
	if s == nil || s.certFile == "" || log == nil || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			remaining, checkErr := certificateFileRemainingValidity(s.certFile)
			if checkErr != nil {
				log.Error("server TLS certificate check failed", zap.Error(checkErr))
			} else if remaining < certificateWarnAfter {
				log.Warn("server TLS certificate will expire soon", zap.Duration("remaining", remaining))
			}
		}
	}
}

func certificateFileRemainingValidity(path string) (time.Duration, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for len(pemData) > 0 {
		block, rest := pem.Decode(pemData)
		pemData = rest
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			return certificateRemainingValidity(tls.Certificate{Certificate: [][]byte{block.Bytes}})
		}
	}
	return 0, errors.New("server TLS certificate contains no valid PEM certificate")
}

// Start 将服务标记为可用，并阻塞处理请求，直到服务被关闭。
func (s *GRPCServer) Start() error {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// Stop 先将健康状态设置为不可用，然后等待正在处理的 RPC 完成。
func (s *GRPCServer) Stop(ctx context.Context) error {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	// GracefulStop 是阻塞方法，必须放入 goroutine，主协程才能同时等待 ctx 超时。
	// done 不传递数据，只负责通知“优雅关闭已经完成”。
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// 所有正在执行的 RPC 都已经正常结束。
		return nil
	case <-ctx.Done():
		// 超过退出期限后强制关闭，再等待 GracefulStop goroutine 真正退出。
		s.server.Stop()
		<-done
		return ctx.Err()
	}
}
