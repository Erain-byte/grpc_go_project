// Package server configures and runs the Gateway's inbound servers.
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	grpcclient "gateway/internal/grpc"
	"gateway/internal/middleware"
	"gateway/internal/ratelimit"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	engine              *gin.Engine
	svcCtx              *svc.ServiceContext
	httpServer          *http.Server
	clientManager       *grpcclient.ClientManager
	jwtMiddleware       *middleware.JWTMiddleware
	sessionMiddleware   *middleware.SessionMiddleware
	rateLimitMiddleware *middleware.RateLimitMiddleware
}

// NewHTTPServer 创建并配置 Gateway 的 HTTP 入口服务。
func NewHTTPServer(svcCtx *svc.ServiceContext, clientManager *grpcclient.ClientManager) (*HTTPServer, error) {
	// 在注册 Consul 前验证证书，避免注册一个无法启动 HTTPS 的实例。
	var tlsConfig *tls.Config
	if svcCtx.Config.HTTP.TLS.Enabled {
		certificate, err := tls.LoadX509KeyPair(svcCtx.Config.HTTP.TLS.CertFile, svcCtx.Config.HTTP.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load Gateway HTTPS certificate: %w", err)
		}
		leaf, err := x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("parse Gateway HTTPS certificate: %w", err)
		}
		now := time.Now()
		if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
			return nil, fmt.Errorf("Gateway HTTPS certificate is expired or not valid yet")
		}
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(slog.Default()))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Tracing(svcCtx.Config.Tracing.ServiceName))
	engine.Use(middleware.LoggerMiddleware(svcCtx.Config.Name))
	corsMiddleware, err := middleware.NewCorsMiddleware(svcCtx.Runtime)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to create CORS middleware", http.StatusInternalServerError)
	}
	engine.Use(corsMiddleware.Handle)
	// AuthService 当前由 Admin 进程实现，ClientManager 通过 Consul 找到其实例。
	/*authClient, err := clientManager.AuthClient(context.Background())
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"failed to create authentication client",
			http.StatusServiceUnavailable,
		)
	}*/
	// JWT 中间件只验证 Token；Session 中间件单独调用 AuthService 检查登录状态。
	jwtMiddleware, jwtErr := middleware.NewJWTMiddleware(svcCtx.Config.Auth)
	if jwtErr != nil {
		return nil, apperror.Wrap(
			jwtErr,
			apperror.CodeInternal,
			"failed to create JWT middleware",
			http.StatusInternalServerError,
		)
	}
	sessionMiddleware, sessionErr := middleware.NewSessionMiddleware(clientManager)
	if sessionErr != nil {
		return nil, apperror.Wrap(
			sessionErr,
			apperror.CodeInternal,
			"failed to create session middleware",
			http.StatusInternalServerError,
		)
	}
	//限流中间件redis
	distributedLimiter, err := ratelimit.NewRedisSlidingWindow(svcCtx.Redis)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to create distributed limiter",
			http.StatusInternalServerError,
		)
	}
	rateLimitMiddleware, err := middleware.NewRateLimitMiddleware(distributedLimiter, svcCtx.Runtime)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to create rate limit middleware",
			http.StatusInternalServerError,
		)
	}
	server := &HTTPServer{
		engine:              engine,
		svcCtx:              svcCtx,
		clientManager:       clientManager,
		jwtMiddleware:       jwtMiddleware,
		sessionMiddleware:   sessionMiddleware,
		rateLimitMiddleware: rateLimitMiddleware,
		httpServer: &http.Server{
			Addr:      fmt.Sprintf("%s:%d", svcCtx.Config.Host, svcCtx.Config.Port),
			Handler:   engine,
			TLSConfig: tlsConfig,
		},
	}
	server.registerRoutes()
	return server, nil
}

func (s *HTTPServer) Start() error {
	var err error
	if s.svcCtx.Config.HTTP.TLS.Enabled {
		// 证书已经在构造时加载到 TLSConfig，不重复读取文件。
		err = s.httpServer.ListenAndServeTLS("", "")
	} else {
		err = s.httpServer.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"HTTP server stopped unexpectedly",
			http.StatusInternalServerError,
		)
	}
	return nil
}

// Shutdown stops accepting new HTTP requests and waits for active handlers.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
