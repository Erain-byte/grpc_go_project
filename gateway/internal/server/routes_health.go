package server

import (
	"net/http"

	"gateway/internal/middleware"
	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
)

func (s *HTTPServer) registerHealthRoutes() {
	s.engine.GET("/health", s.healthCheck)
}

func (s *HTTPServer) healthCheck(c *gin.Context) {
	// Consul Agent 会按照注册时配置的周期请求此路由。
	if s.svcCtx == nil {
		middleware.Fail(c, apperror.Unavailable("service context is not initialized"))
		return
	}

	// Redis 允许降级时，Redis 暂时不可用不应让 Consul 摘除 Gateway；
	// 生产环境或明确要求 Redis 健康时，Redis 才属于就绪条件。
	redisRequired := s.svcCtx.Config.Redis.HealthRequired || s.svcCtx.Config.IsProduction()
	if !redisRequired {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	if s.svcCtx.Redis == nil {
		middleware.Fail(c, apperror.Unavailable("Redis client is not initialized"))
		return
	}

	// 直接使用 HTTP 请求的 Context：Consul 取消请求或超时后，Ping 也会取消。
	if err := s.svcCtx.Redis.Ping(c.Request.Context()); err != nil {
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
