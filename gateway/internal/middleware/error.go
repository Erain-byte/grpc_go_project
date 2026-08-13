package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const requestIDHeader = RequestIDHeader

// Fail 将业务错误交给统一错误中间件，并停止执行后续处理函数。
func Fail(c *gin.Context, err error) { // nolint: revive
	_ = c.Error(err) // will be handled by ErrorHandler
	c.Abort()        // will stop the remaining handlers
}

// ErrorResponse 是 HTTP 接口统一返回的错误结构。
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      apperror.Code `json:"code"`
	Message   string        `json:"message"`
	Details   any           `json:"details,omitempty"`
	RequestID string        `json:"request_id,omitempty"`
}

// ErrorHandler 统一处理业务错误和未捕获的 panic。
// 该中间件只需通过 Engine.Use 注册一次。
func ErrorHandler(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(c.Request.Context(), "recovered HTTP panic",
					"panic", recovered,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"request_id", c.Request.Header.Get(requestIDHeader),
					"stack", string(debug.Stack()),
				)
				if !c.Writer.Written() {
					WriteError(c.Writer, c.Request, apperror.New(
						apperror.CodeInternal,
						"internal server error",
						http.StatusInternalServerError,
					))
				}
				c.Abort()
			}
		}()

		c.Next() // process next middleware

		ginErr := c.Errors.Last()
		if ginErr == nil {
			return
		}
		logger.ErrorContext(c.Request.Context(), "HTTP request failed",
			"error", ginErr.Err,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"request_id", c.Request.Header.Get(requestIDHeader),
		)
		if !c.Writer.Written() {
			WriteError(c.Writer, c.Request, ginErr.Err)
		}
	}
}

// WriteError 将错误转换为统一的 JSON 响应。
// 未知错误统一返回 INTERNAL_ERROR，避免泄露内部实现细节。
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	appErr := FromGRPCError(err)
	if appErr == nil {
		return
	}

	status := appErr.HTTPStatus
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	body := ErrorResponse{Error: ErrorBody{
		Code:      appErr.Code,
		Message:   appErr.Message,
		Details:   appErr.Details,
		RequestID: r.Header.Get(requestIDHeader),
	}}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// 响应头已经发出，客户端断开连接时无需再次处理编码错误。
	_ = json.NewEncoder(w).Encode(body)
}

// FromGRPCError 将 gRPC 状态码转换为统一的应用错误。
// 无法识别的错误统一按内部错误处理，避免泄露内部信息。
func FromGRPCError(err error) *apperror.Error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr
	}

	grpcStatus, ok := status.FromError(err)
	if !ok {
		return apperror.As(err)
	}
	message := grpcStatus.Message()
	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		return apperror.Wrap(err, apperror.CodeInvalidArgument, message, http.StatusBadRequest)
	case codes.Unauthenticated:
		return apperror.Wrap(err, apperror.CodeUnauthorized, message, http.StatusUnauthorized)
	case codes.PermissionDenied:
		return apperror.Wrap(err, apperror.CodeForbidden, message, http.StatusForbidden)
	case codes.NotFound:
		return apperror.Wrap(err, apperror.CodeNotFound, message, http.StatusNotFound)
	case codes.AlreadyExists, codes.Aborted:
		return apperror.Wrap(err, apperror.CodeConflict, message, http.StatusConflict)
	case codes.ResourceExhausted:
		return apperror.Wrap(err, apperror.CodeTooManyRequests, message, http.StatusTooManyRequests)
	case codes.DeadlineExceeded:
		return apperror.Wrap(err, apperror.CodeTimeout, "request timed out", http.StatusGatewayTimeout)
	case codes.Canceled:
		return apperror.Wrap(err, apperror.CodeTimeout, "request canceled", http.StatusRequestTimeout)
	case codes.Unavailable:
		return apperror.Wrap(err, apperror.CodeUnavailable, "service is unavailable", http.StatusServiceUnavailable)
	case codes.Unimplemented:
		return apperror.Wrap(err, apperror.CodeInternal, "service method is not implemented", http.StatusNotImplemented)
	default:
		return apperror.Wrap(err, apperror.CodeInternal, "internal server error", http.StatusInternalServerError)
	}
}
