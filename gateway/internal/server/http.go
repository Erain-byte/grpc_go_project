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
	engine        *gin.Engine
	svcCtx        *svc.ServiceContext
	httpServer    *http.Server
	clientManager *grpcclient.ClientManager
}

// NewHTTPServer 创建并配置 Gateway 的 HTTP 入口服务。
func NewHTTPServer(svcCtx *svc.ServiceContext, clientManager *grpcclient.ClientManager) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(slog.Default()))

	server := &HTTPServer{
		engine:        engine,
		svcCtx:        svcCtx,
		clientManager: clientManager,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", svcCtx.Config.Host, svcCtx.Config.Port),
			Handler: engine,
		},
	}
	server.registerRoutes()
	return server
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
