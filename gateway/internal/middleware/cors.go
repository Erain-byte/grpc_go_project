package middleware

import (
	"fmt"
	"gateway/internal/runtimeconfig"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type CorsMiddleware struct{ store *runtimeconfig.Store }

func NewCorsMiddleware(store *runtimeconfig.Store) (*CorsMiddleware, error) {
	if store == nil {
		return nil, fmt.Errorf("CORS runtime config store is nil")
	}
	return &CorsMiddleware{store: store}, nil
}

func (m *CorsMiddleware) Handle(c *gin.Context) {
	snapshot := m.store.Load()
	if snapshot == nil {
		Fail(c, fmt.Errorf("runtime config is unavailable"))
		return
	}
	// 单个请求只读取一次快照，避免一次响应混用新旧配置。
	cfg := snapshot.CORS
	origin := c.GetHeader("Origin")
	if origin != "" {
		c.Header("Vary", "Origin")
	}
	if allowedOrigin := resolveAllowedOrigin(origin, cfg); allowedOrigin != "" {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
	}
	if len(cfg.AllowMethods) > 0 {
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
	}
	if len(cfg.AllowHeaders) > 0 {
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
	}
	if len(cfg.ExposeHeaders) > 0 {
		c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
	}
	if cfg.AllowCredentials {
		c.Header("Access-Control-Allow-Credentials", "true")
	}
	if cfg.MaxAge > 0 {
		c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	}
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}

func resolveAllowedOrigin(origin string, cfg runtimeconfig.CORSRule) string {
	if origin == "" {
		return ""
	}
	for _, allowed := range cfg.AllowOrigins {
		if allowed == "*" {
			if cfg.AllowCredentials {
				return origin
			}
			return "*"
		}
		if allowed == origin {
			return origin
		}
	}
	return ""
}
