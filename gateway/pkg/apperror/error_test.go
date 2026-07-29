package apperror

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAsPreservesApplicationError(t *testing.T) {
	want := NotFound("user not found")
	got := As(want)
	if got != want {
		t.Fatalf("As() returned a different error: %#v", got)
	}
}

func TestToGRPC(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{name: "nil", err: nil, wantCode: codes.OK},
		{name: "invalid argument", err: InvalidArgument("invalid name"), wantCode: codes.InvalidArgument, wantMsg: "invalid name"},
		{name: "unauthorized", err: Unauthorized("login required"), wantCode: codes.Unauthenticated, wantMsg: "login required"},
		{name: "forbidden", err: Forbidden("access denied"), wantCode: codes.PermissionDenied, wantMsg: "access denied"},
		{name: "not found", err: NotFound("user not found"), wantCode: codes.NotFound, wantMsg: "user not found"},
		{name: "conflict", err: Conflict("already exists"), wantCode: codes.AlreadyExists, wantMsg: "already exists"},
		{name: "rate limit", err: TooManyRequests("slow down"), wantCode: codes.ResourceExhausted, wantMsg: "slow down"},
		{name: "unavailable", err: Unavailable("service unavailable"), wantCode: codes.Unavailable, wantMsg: "service unavailable"},
		{name: "timeout", err: Timeout("operation timed out"), wantCode: codes.DeadlineExceeded, wantMsg: "operation timed out"},
		{name: "canceled context", err: context.Canceled, wantCode: codes.Canceled, wantMsg: "request canceled"},
		{name: "deadline context", err: context.DeadlineExceeded, wantCode: codes.DeadlineExceeded, wantMsg: "request timed out"},
		{name: "unknown", err: errors.New("database password leaked"), wantCode: codes.Internal, wantMsg: "internal server error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ToGRPC(test.err)
			if status.Code(got) != test.wantCode {
				t.Fatalf("ToGRPC() code = %v, want %v", status.Code(got), test.wantCode)
			}
			if got != nil && status.Convert(got).Message() != test.wantMsg {
				t.Fatalf("ToGRPC() message = %q, want %q", status.Convert(got).Message(), test.wantMsg)
			}
		})
	}
}

func TestToGRPCPreservesStatusError(t *testing.T) {
	want := status.Error(codes.Aborted, "transaction aborted")
	if got := ToGRPC(want); got != want {
		t.Fatalf("ToGRPC() did not preserve status error: %v", got)
	}
}

func TestAsHidesUnknownError(t *testing.T) {
	cause := errors.New("database password leaked")
	got := As(cause)
	if got.Code != CodeInternal || got.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected mapping: %#v", got)
	}
	if got.Message != "internal server error" {
		t.Fatalf("private cause exposed: %q", got.Message)
	}
	if !errors.Is(got, cause) {
		t.Fatal("internal cause was not retained for logging")
	}
}
