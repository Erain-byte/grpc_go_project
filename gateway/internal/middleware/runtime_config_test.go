package middleware

import (
	"context"
	"gateway/internal/ratelimit"
	"gateway/internal/runtimeconfig"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type capturingLimiter struct {
	limit  int64
	window time.Duration
}

func (l *capturingLimiter) Allow(_ context.Context, _ string, limit int64, window time.Duration) (ratelimit.Result, error) {
	l.limit, l.window = limit, window
	return ratelimit.Result{Allowed: true, Limit: limit, Remaining: limit - 1}, nil
}

func TestCorsMiddlewareUsesUpdatedSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := runtimeconfig.NewStore(testSnapshot("https://old.example", 10))
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := NewCorsMiddleware(store)
	if err != nil {
		t.Fatal(err)
	}
	request := func(origin string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
		ctx.Request.Header.Set("Origin", origin)
		middleware.Handle(ctx)
		return recorder
	}
	if got := request("https://old.example").Header().Get("Access-Control-Allow-Origin"); got != "https://old.example" {
		t.Fatalf("old CORS origin = %q", got)
	}
	if err := store.Store(testSnapshot("https://new.example", 20)); err != nil {
		t.Fatal(err)
	}
	if got := request("https://new.example").Header().Get("Access-Control-Allow-Origin"); got != "https://new.example" {
		t.Fatalf("updated CORS origin = %q", got)
	}
}

func TestRateLimitMiddlewareUsesUpdatedSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, _ := runtimeconfig.NewStore(testSnapshot("https://example.com", 10))
	limiter := &capturingLimiter{}
	middleware, err := NewRateLimitMiddleware(limiter, store)
	if err != nil {
		t.Fatal(err)
	}
	invoke := func() {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
		middleware.Handle(ctx)
	}
	invoke()
	if limiter.limit != 10 {
		t.Fatalf("initial limit = %d", limiter.limit)
	}
	if err := store.Store(testSnapshot("https://example.com", 25)); err != nil {
		t.Fatal(err)
	}
	invoke()
	if limiter.limit != 25 {
		t.Fatalf("updated limit = %d", limiter.limit)
	}
}

func testSnapshot(origin string, limit int64) *runtimeconfig.Snapshot {
	return &runtimeconfig.Snapshot{
		Version: 1,
		CORS: runtimeconfig.CORSRule{
			AllowOrigins: []string{origin},
			AllowMethods: []string{http.MethodGet},
		},
		RateLimit: runtimeconfig.RateLimitRule{
			Enabled:      true,
			Limit:        limit,
			Window:       time.Minute,
			RedisTimeout: time.Second,
		},
	}
}
