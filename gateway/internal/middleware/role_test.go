package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireAnyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		claims     *AccessTokenClaims
		wantStatus int
	}{
		{name: "allowed", claims: &AccessTokenClaims{RealmAccess: RealmAccess{Roles: []string{"admin"}}}, wantStatus: http.StatusNoContent},
		{name: "forbidden", claims: &AccessTokenClaims{RealmAccess: RealmAccess{Roles: []string{"user"}}}, wantStatus: http.StatusForbidden},
		{name: "missing claims", wantStatus: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(ErrorHandler(nil))
			engine.GET("/protected", func(c *gin.Context) {
				if tt.claims != nil {
					c.Set(ContextClaims, tt.claims)
				}
				c.Next()
			}, RequireAnyRole("admin", "super-admin"), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
