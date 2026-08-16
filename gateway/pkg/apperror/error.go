// Package apperror defines errors that can safely cross an API boundary.
package apperror

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Code is a stable, machine-readable error code.
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

// Error carries the public API representation of an error and its private cause.
// Cause is available to logs through Unwrap but is never serialized directly.
type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Details    any
	Cause      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New creates an application error. Prefer the named constructors below.
func New(code Code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

// Wrap attaches an internal cause without exposing it to clients.
func Wrap(err error, code Code, message string, status int) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: message, HTTPStatus: status, Cause: err}
}

// Named constructors
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

// As returns err as an application error, or a safe internal error otherwise.
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

// ToGRPC converts an application error to a gRPC status error.
// Existing gRPC status errors are returned unchanged. Unknown errors are
// deliberately exposed as a generic Internal error so implementation details
// do not leak to clients.
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
