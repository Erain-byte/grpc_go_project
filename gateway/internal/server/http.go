// Package server configures and runs the Gateway's inbound servers.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	grpcclient "gateway/internal/grpc"
	"gateway/internal/middleware"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	engine            *gin.Engine
	svcCtx            *svc.ServiceContext
	httpServer        *http.Server
	clientManager     *grpcclient.ClientManager
	jwtMiddleware     *middleware.JWTMiddleware
	sessionMiddleware *middleware.SessionMiddleware
}

// NewHTTPServer 创建并配置 Gateway 的 HTTP 入口服务。
func NewHTTPServer(svcCtx *svc.ServiceContext, clientManager *grpcclient.ClientManager) (*HTTPServer, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(slog.Default()))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Tracing(svcCtx.Config.Tracing.ServiceName))
	engine.Use(middleware.LoggerMiddleware(svcCtx.Config.Name))
	CorsMiddelware := middleware.NewCorsMiddleware(svcCtx.Config.Cors)
	engine.Use(CorsMiddelware.Handle)
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
	server := &HTTPServer{
		engine:            engine,
		svcCtx:            svcCtx,
		clientManager:     clientManager,
		jwtMiddleware:     jwtMiddleware,
		sessionMiddleware: sessionMiddleware,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", svcCtx.Config.Host, svcCtx.Config.Port),
			Handler: engine,
		},
	}
	server.registerRoutes()
	return server, nil
}

func (s *HTTPServer) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
