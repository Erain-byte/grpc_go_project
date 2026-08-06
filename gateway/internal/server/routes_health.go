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
