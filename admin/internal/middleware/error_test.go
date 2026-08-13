package middleware

import (
	"admin/pkg/apperorr"
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{name: "application error", err: apperorr.NotFound("admin not found"), wantCode: codes.NotFound},
		{name: "deadline", err: context.DeadlineExceeded, wantCode: codes.DeadlineExceeded},
		{name: "canceled", err: context.Canceled, wantCode: codes.Canceled},
		{name: "existing status", err: status.Error(codes.Aborted, "aborted"), wantCode: codes.Aborted},
		{name: "unknown error", err: errors.New("database details"), wantCode: codes.Internal},
	}

	interceptor := ErrorHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := interceptor(
				context.Background(),
				nil,
				&grpc.UnaryServerInfo{FullMethod: "/admin.v1.AdminService/Test"},
				func(context.Context, any) (any, error) { return nil, tt.err },
			)
			if got := status.Code(err); got != tt.wantCode {
				t.Fatalf("status code = %v, want %v (err=%v)", got, tt.wantCode, err)
			}
		})
	}
}

func TestErrorHandlerReturnsResponse(t *testing.T) {
	want := struct{ Value string }{Value: "ok"}
	got, err := ErrorHandler()(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/admin.v1.AdminService/Test"},
		func(context.Context, any) (any, error) { return want, nil },
	)
	if err != nil {
		t.Fatalf("ErrorHandler() error = %v", err)
	}
	if got != want {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
}
