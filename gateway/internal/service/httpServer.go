package service

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"gateway/internal/middleware"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine     *gin.Engine
	svcCtx     *svc.ServiceContext
	httpServer *http.Server
}

// NewServer 创建并配置 Gateway 的 HTTP 入口服务。
func NewServer(svcCtx *svc.ServiceContext) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(slog.Default()))

	server := &Server{
		engine: engine,
		svcCtx: svcCtx,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", svcCtx.Config.Host, svcCtx.Config.Port),
			Handler: engine,
		},
	}
	server.registerRoutes()
	return server
}

// registerRoutes 注册 Gateway 自身提供的 HTTP 路由。
// 业务接口将在这里接入 gRPC Client 或 grpc-gateway，不再转发到下游 HTTP 服务。
func (s *Server) registerRoutes() {
	s.engine.GET("/health", s.healthCheck)
}

func (s *Server) healthCheck(c *gin.Context) {
	if err := s.svcCtx.HealthCheck(c.Request.Context()); err != nil {
		middleware.Fail(c, apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"service is unhealthy",
			http.StatusServiceUnavailable,
		))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) Start() error {
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
