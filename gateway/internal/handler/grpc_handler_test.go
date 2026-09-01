package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gateway/internal/middleware"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

func TestWithMetadataPropagatesVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/info", nil)
	ctx.Set(middleware.ContextUserID, "2")
	ctx.Set(middleware.ContextRole, "admin")
	ctx.Set(middleware.ContextSessionID, "session-1")
	ctx.Set(middleware.ContextTokenID, "token-1")

	md, ok := metadata.FromOutgoingContext(WithMetadata(ctx))
	if !ok {
		t.Fatal("outgoing metadata is missing")
	}
	checks := map[string]string{
		"x-user-id":    "2",
		"x-user-role":  "admin",
		"x-session-id": "session-1",
		"x-token-id":   "token-1",
	}
	for key, want := range checks {
		values := md.Get(key)
		if len(values) != 1 || values[0] != want {
			t.Fatalf("metadata %s = %v, want %q", key, values, want)
		}
	}
}

func TestWithMetadataDoesNotAddEmptyPublicIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	if _, ok := metadata.FromOutgoingContext(WithMetadata(ctx)); ok {
		t.Fatal("public request unexpectedly contains outgoing metadata")
	}
}
