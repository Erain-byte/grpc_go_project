package server

import (
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

	"user/internal/config"
	"user/internal/handler"
	"user/internal/middleware"
	"user/internal/svc"

	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type GrpcServer struct {
	server   *grpc.Server
	health   *health.Server
	listener net.Listener
	certFile string
}

// 证书过期时间
const certificateWarnAfter = 24 * time.Hour

func NewGrpcServer(cfg *config.Config, svcCtx *svc.ServiceContext) (*GrpcServer, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.GRPCPort)))
	if err != nil {
		return nil, err
	}
	//初始化AUTH拦截器
	authInterceptor, err := middleware.NewAuthInterceptor(cfg.Auth)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	//初始化rate_limt拦截器
	rateLimitInterceptor := middleware.NewRateLimit(
		cfg.RateLimit.Enabled,
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.Burst,
		cfg.RateLimit.MaxUnaryConcurrency,
		cfg.RateLimit.MaxStreamConcurrency,
	)

	option := []grpc.ServerOption{
		// StatsHandler 从 gRPC metadata 提取上游 Trace，并记录服务端 Span。
		//grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.StatsHandler(middleware.NewServerStatsHandler()),
		//注册多个一元拦截器
		grpc.ChainUnaryInterceptor(
			middleware.RequestID(),
			middleware.ErrorHandler(),
			authInterceptor.Unary(),
			rateLimitInterceptor.Unary(),
		),

		//注册多个流式拦截器
		grpc.ChainStreamInterceptor(
			authInterceptor.Stream(),
			rateLimitInterceptor.Stream(),
		),
		//grpc.StreamInterceptor(authInterceptor.Stream()),
	}
	//开启TLS
	if cfg.GRPC.UseTLS {
		creds, remaining, credentialErr := serverTLS(&cfg.GRPC)
		if credentialErr != nil {
			//获取证书异常关闭监听
			_ = listener.Close()
			return nil, credentialErr
		}
		//设置证书
		if remaining < certificateWarnAfter {
			svcCtx.Logger.Warn(
				"server TLS certificate will expire soon",
				zap.Duration("remaining", remaining),
			)
		}
		option = append(option, grpc.Creds(creds))
	}
	//创建grpc服务
	grpcServer := grpc.NewServer(option...)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	//注册到GRPC服务
	userv1.RegisterUserServiceServer(grpcServer, handler.NewUserHandler(svcCtx))
	return &GrpcServer{server: grpcServer, health: healthServer, listener: listener}, nil
}

func (s *GrpcServer) Start() error {
	//设置服务状态为服务中
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// 优雅关闭grpc 服务
func (s *GrpcServer) Stop(ctx context.Context) error {
	//设置服务状态为未服务
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	done := make(chan struct{})
	go func() { s.server.GracefulStop(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		<-done
		return ctx.Err()
	}
}

// MonitorCertificate 定期检查服务端证书；过期记录错误，临期只记录警告。
func (s *GrpcServer) MonitorCertificate(ctx context.Context, interval time.Duration, log *zap.Logger) {
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

// 证书创建服务端 TLS 凭证；配置 ClientCAFile 时启用双向 TLS。
func serverTLS(cfg *config.GRPCConfig) (credentials.TransportCredentials, time.Duration, error) {
	//加载服务端证书和私钥
	certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, 0, fmt.Errorf("loadd server TLS certificate: %w", err)
	}
	//判断证书过期时间
	remaining, err := checkCertificateExpiration(certificate)
	if err != nil {
		return nil, remaining, err
	}
	//配置TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		//最低版本
		MinVersion: tls.VersionTLS12,
	}
	//是否开启双向认证
	if cfg.ClientCAFile != "" {
		pemData, err := os.ReadFile(strings.TrimSpace(cfg.ClientCAFile))
		if err != nil {
			return nil, 0, fmt.Errorf("read client CA file: %w", err)
		}
		//证书连接池
		caPool := x509.NewCertPool()
		//将证书添加到连接池,如果证书无效，则返回false
		if !caPool.AppendCertsFromPEM(pemData) {
			return nil, 0, errors.New("failed to append certificates")
		}
		//
		tlsConfig.ClientCAs = caPool
		//是否验证客户端证书
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return credentials.NewTLS(tlsConfig), remaining, nil
}

// 判断证书过期时间
func checkCertificateExpiration(cer tls.Certificate) (time.Duration, error) {
	if len(cer.Certificate) == 0 {
		return 0, errors.New("no certificate provided")
	}
	leaf, err := x509.ParseCertificate(cer.Certificate[0])
	if err != nil {
		return 0, fmt.Errorf("parse server TLS certificate: %w", err)
	}
	//查看证书是否生效
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return 0, fmt.Errorf("TLS certificate is not valid until %s", leaf.NotBefore)
	}
	//查看证书是否过期
	if leaf.NotAfter.Sub(now) <= 0 {
		return leaf.NotAfter.Sub(now), fmt.Errorf("TLS certificate has expired on %s", leaf.NotAfter)
	}
	return leaf.NotAfter.Sub(now), nil
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
			return checkCertificateExpiration(tls.Certificate{Certificate: [][]byte{block.Bytes}})
		}
	}
	return 0, errors.New("server TLS certificate contains no valid PEM certificate")
}
