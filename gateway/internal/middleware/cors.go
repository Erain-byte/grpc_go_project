package middleware

import (
	"fmt"
	"gateway/internal/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type CorsMiddleware struct {
	cfg config.CORSConfig
}

func NewCorsMiddleware(cfg config.CORSConfig) *CorsMiddleware {
	return &CorsMiddleware{
		cfg: cfg,
	}
}
func (s *CorsMiddleware) Handle(c *gin.Context) {

	origin := c.Request.Header.Get("Origin")
	if origin != "" {
		c.Header("Vary", "Origin") // 添加Vary头，以便代理服务器正确缓存响应
	}
	if allowedOrigin := resolveAllowedOrigin(origin, s.cfg); allowedOrigin != "" {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
	}
	if allowedOrigin := resolveAllowedOrigin(origin, s.cfg); allowedOrigin != "" {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
	}

	if len(s.cfg.AllowMethods) > 0 {
		c.Header("Access-Control-Allow-Methods", strings.Join(s.cfg.AllowMethods, ", "))
	}

	if len(s.cfg.AllowHeaders) > 0 {
		c.Header("Access-Control-Allow-Headers", strings.Join(s.cfg.AllowHeaders, ", "))
	}

	if len(s.cfg.ExposeHeaders) > 0 {
		c.Header("Access-Control-Expose-Headers", strings.Join(s.cfg.ExposeHeaders, ", "))
	}

	if s.cfg.AllowCredentials {
		c.Header("Access-Control-Allow-Credentials", "true")
	}

	if s.cfg.MaxAge > 0 {
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", s.cfg.MaxAge))
	}

	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}
func resolveAllowedOrigin(origin string, cfg config.CORSConfig) string {
	if len(cfg.AllowOrigins) == 0 {
		return ""
	}
	if contains(cfg.AllowOrigins, "*") {
		if cfg.AllowCredentials {
			return origin
		}
		return "*"
	}
	if contains(cfg.AllowOrigins, origin) {
		return origin
	}
	return ""
}

// 确定是否允许跨域
func contains(slices []string, e string) bool {
	for _, a := range slices {
		if a == e {
			return true
		}
	}
	return false
}
