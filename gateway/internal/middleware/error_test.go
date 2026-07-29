package middleware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorHandlerWritesApplicationError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine.Use(ErrorHandler(logger))
	engine.GET("/users/:id", func(c *gin.Context) {
		_ = c.Error(apperror.NotFound("user not found"))
		c.Abort()
	})

	request := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	request.Header.Set(requestIDHeader, "request-42")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != apperror.CodeNotFound || body.Error.Message != "user not found" {
		t.Fatalf("error body = %#v", body.Error)
	}
	if body.Error.RequestID != "request-42" {
		t.Fatalf("request ID = %q, want %q", body.Error.RequestID, "request-42")
	}
}

func TestErrorHandlerLeavesSuccessfulResponseUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ErrorHandler(nil))
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}" {
		t.Fatalf("response = (%d, %q), want (200, status ok)", response.Code, response.Body.String())
	}
}

func TestErrorHandlerRecoversPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine.Use(ErrorHandler(logger))
	engine.GET("/panic", func(c *gin.Context) {
		panic("unexpected failure")
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != apperror.CodeInternal || body.Error.Message != "internal server error" {
		t.Fatalf("error body = %#v", body.Error)
	}
}

func TestFromGRPCError(t *testing.T) {
	tests := []struct {
		name       string
		grpcCode   codes.Code
		wantCode   apperror.Code
		wantStatus int
	}{
		{name: "invalid argument", grpcCode: codes.InvalidArgument, wantCode: apperror.CodeInvalidArgument, wantStatus: http.StatusBadRequest},
		{name: "unauthenticated", grpcCode: codes.Unauthenticated, wantCode: apperror.CodeUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "permission denied", grpcCode: codes.PermissionDenied, wantCode: apperror.CodeForbidden, wantStatus: http.StatusForbidden},
		{name: "not found", grpcCode: codes.NotFound, wantCode: apperror.CodeNotFound, wantStatus: http.StatusNotFound},
		{name: "already exists", grpcCode: codes.AlreadyExists, wantCode: apperror.CodeConflict, wantStatus: http.StatusConflict},
		{name: "resource exhausted", grpcCode: codes.ResourceExhausted, wantCode: apperror.CodeTooManyRequests, wantStatus: http.StatusTooManyRequests},
		{name: "deadline", grpcCode: codes.DeadlineExceeded, wantCode: apperror.CodeTimeout, wantStatus: http.StatusGatewayTimeout},
		{name: "unavailable", grpcCode: codes.Unavailable, wantCode: apperror.CodeUnavailable, wantStatus: http.StatusServiceUnavailable},
		{name: "internal", grpcCode: codes.Internal, wantCode: apperror.CodeInternal, wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := FromGRPCError(status.Error(test.grpcCode, "gRPC detail"))
			if got.Code != test.wantCode || got.HTTPStatus != test.wantStatus {
				t.Fatalf("FromGRPCError() = (%q, %d), want (%q, %d)", got.Code, got.HTTPStatus, test.wantCode, test.wantStatus)
			}
		})
	}
}

func TestWriteErrorConvertsGRPCError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	response := httptest.NewRecorder()
	WriteError(response, request, status.Error(codes.NotFound, "user not found"))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != apperror.CodeNotFound || body.Error.Message != "user not found" {
		t.Fatalf("error body = %#v", body.Error)
	}
}
