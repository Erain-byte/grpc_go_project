// Package apperror 定义 Web3 服务内部统一使用的业务错误，
// 并负责在 gRPC 边界将业务错误转换为稳定的 status error。
package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Code 是与具体传输协议无关的业务错误码。
type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeInternal        Code = "INTERNAL_ERROR"
	CodeUnavailable     Code = "SERVICE_UNAVAILABLE"
	CodeTimeout         Code = "TIMEOUT"
)

// Error 保存对外稳定信息和仅供服务内部排查的原始错误。
// Cause 不会由 ToGRPC 暴露给调用方。
type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Details    any
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 让 errors.Is 和 errors.As 能继续检查底层错误。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(code Code, message string, httpStatus int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap 保留底层错误用于日志和 Trace；err 为 nil 时仍返回 nil。
func Wrap(err error, code Code, message string, httpStatus int) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Cause:      err,
	}
}

func InvalidArgument(message string) *Error {
	return New(CodeInvalidArgument, message, http.StatusBadRequest)
}

func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, message, http.StatusUnauthorized)
}

func Forbidden(message string) *Error {
	return New(CodeForbidden, message, http.StatusForbidden)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, message, http.StatusNotFound)
}

func Conflict(message string) *Error {
	return New(CodeConflict, message, http.StatusConflict)
}

func TooManyRequests(message string) *Error {
	return New(CodeTooManyRequests, message, http.StatusTooManyRequests)
}

func Unavailable(message string) *Error {
	return New(CodeUnavailable, message, http.StatusServiceUnavailable)
}

func Timeout(message string) *Error {
	return New(CodeTimeout, message, http.StatusGatewayTimeout)
}

// As 将任意错误规范化为 *Error。未知错误统一隐藏为内部错误。
func As(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return &Error{
		Code:       CodeInternal,
		Message:    "internal server error",
		HTTPStatus: http.StatusInternalServerError,
		Cause:      err,
	}
}

// ToGRPC 在服务边界将业务错误转换为 gRPC status error。
// 已经是 status error 的错误保持不变，未知错误不会向调用方泄露内部细节。
func ToGRPC(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request timed out")
	}

	var appErr *Error
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, "internal server error")
	}
	return status.Error(grpcCode(appErr.Code), appErr.Message)
}

func grpcCode(code Code) codes.Code {
	switch code {
	case CodeInvalidArgument:
		return codes.InvalidArgument
	case CodeUnauthorized:
		return codes.Unauthenticated
	case CodeForbidden:
		return codes.PermissionDenied
	case CodeNotFound:
		return codes.NotFound
	case CodeConflict:
		return codes.AlreadyExists
	case CodeTooManyRequests:
		return codes.ResourceExhausted
	case CodeUnavailable:
		return codes.Unavailable
	case CodeTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}
