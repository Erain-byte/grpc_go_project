package apperorr

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause) // 500 + 内部错误
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message) // 500
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause // 返回内部错误
}

func New(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status} // 500
}

func Wrap(err error, code Code, message string, status int) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: err} // 500
}

func InvalidArgument(message string) *Error {
	return New(CodeInvalidArgument, message, http.StatusBadRequest) // 400
}
func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, message, http.StatusUnauthorized) // 401
}
func Forbidden(message string) *Error {
	return New(CodeForbidden, message, http.StatusForbidden) // 403
}
func NotFound(message string) *Error {
	return New(CodeNotFound, message, http.StatusNotFound) // 404
}
func Conflict(message string) *Error {
	return New(CodeConflict, message, http.StatusConflict) // 409
}
func TooManyRequests(message string) *Error {
	return New(CodeTooManyRequests, message, http.StatusTooManyRequests) // 429
}
func Unavailable(message string) *Error {
	return New(CodeUnavailable, message, http.StatusServiceUnavailable) // 503
}
func Timeout(message string) *Error {
	return New(CodeTimeout, message, http.StatusGatewayTimeout) // 504
}

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
