package middleware

import (
	"time"

	"gateway/internal/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggerMiddleware records one structured access log after each HTTP request.
func LoggerMiddleware(serviceName string) gin.HandlerFunc {
	return loggerMiddleware(logger.Logger, serviceName)
}

func loggerMiddleware(zapLogger *zap.Logger, serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		statusCode := c.Writer.Status()
		ginErr := c.Errors.Last()
		// ErrorHandler is the outer middleware and writes the final response after
		// this middleware returns. Derive the intended status for this log entry.
		if ginErr != nil && statusCode < 400 {
			statusCode = FromGRPCError(ginErr.Err).HTTPStatus
		}

		fields := []zap.Field{
			zap.String("service", serviceName),
			zap.String("trace_id", logger.GetTraceID(c.Request.Context())),
			zap.String("request_id", c.GetString(RequestIDKey)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status_code", statusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("response_size", c.Writer.Size()),
		}
		if userID := c.GetString(ContextUserID); userID != "" {
			fields = append(fields, zap.String("user_id", userID))
		}
		if ginErr != nil {
			fields = append(fields, zap.Error(ginErr.Err))
		}

		switch {
		case statusCode >= 500:
			zapLogger.Error("HTTP request completed", fields...)
		case statusCode >= 400:
			zapLogger.Warn("HTTP request completed", fields...)
		default:
			zapLogger.Info("HTTP request completed", fields...)
		}
	}
}
