package handler

import (
	"net/http/httptest"
	"testing"

	"gateway/internal/middleware"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

func TestWithMetadataPropagatesRequestIDForPublicRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/tron/usdt/approvals/check", nil)
	ctx.Set(middleware.RequestIDKey, "request-42")

	outgoing := WithMetadata(ctx)
	md, ok := metadata.FromOutgoingContext(outgoing)
	if !ok || len(md.Get("x-request-id")) != 1 || md.Get("x-request-id")[0] != "request-42" {
		t.Fatalf("outgoing request ID = %#v", md.Get("x-request-id"))
	}
	if len(md.Get("authorization")) != 0 {
		t.Fatalf("public request unexpectedly contains authorization metadata")
	}
}
