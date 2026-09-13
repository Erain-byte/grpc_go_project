package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 验证 Handler 的真实 HTTP 输出，防止内部限流错误被误转换为 500。
func TestWriteGrpcErrorResourceExhausted(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	const message = "too many requests"
	writeGrpcError(ctx, status.Error(codes.ResourceExhausted, message))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("HTTP status = %d, want 429", recorder.Code)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Message != message {
		t.Fatalf("message = %q, want %q", body.Message, message)
	}
}
