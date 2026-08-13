package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func testJWTMiddleware(t *testing.T) *JWTMiddleware {
	t.Helper()
	middleware, err := NewJWTMiddleware(config.AuthConfig{
		AccessToken: config.AccessTokenConfig{
			Issuers:   []string{"admin-service", "user-service"},
			Audience:  "gateway",
			Algorithm: "HS256",
			Secret:    testJWTSecret,
		},
	})
	if err != nil {
		t.Fatalf("NewJWTMiddleware() error = %v", err)
	}
	return middleware
}

func signedAccessToken(
	t *testing.T,
	issuer string,
	audience string,
	expiresAt time.Time,
) string {
	t.Helper()
	claims := AccessTokenClaims{
		Role:      "admin",
		SessionID: "session-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "admin-1",
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return tokenString
}

func TestExtractBearerToken(t *testing.T) {
	got, err := extractBearerToken("Bearer token-value")
	if err != nil {
		t.Fatalf("extractBearerToken() error = %v", err)
	}
	if got != "token-value" {
		t.Fatalf("token = %q, want token-value", got)
	}
}

func TestValidateJWT(t *testing.T) {
	middleware := testJWTMiddleware(t)
	tokenString := signedAccessToken(
		t,
		"admin-service",
		"gateway",
		time.Now().Add(time.Hour),
	)

	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}
	if claims.Subject != "admin-1" {
		t.Fatalf("subject = %q, want admin-1", claims.Subject)
	}
	if claims.SessionID != "session-1" {
		t.Fatalf("session ID = %q, want session-1", claims.SessionID)
	}
}

func TestValidateJWTRejectsUnknownIssuer(t *testing.T) {
	middleware := testJWTMiddleware(t)
	tokenString := signedAccessToken(
		t,
		"unknown-service",
		"gateway",
		time.Now().Add(time.Hour),
	)

	if _, err := middleware.ValidateJWT(tokenString); err == nil {
		t.Fatal("ValidateJWT() should reject an unknown issuer")
	}
}

func TestJWTMiddlewareOnlyProtectsConfiguredRouteGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtMiddleware := testJWTMiddleware(t)
	engine := gin.New()
	engine.Use(ErrorHandler(nil))

	engine.GET("/public", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	protected := engine.Group("/protected", jwtMiddleware.Handle)
	protected.GET("", func(c *gin.Context) {
		userID, _ := c.Get(ContextUserID)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	publicRecorder := httptest.NewRecorder()
	publicRequest := httptest.NewRequest(http.MethodGet, "/public", nil)
	engine.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusNoContent {
		t.Fatalf("public status = %d, want %d", publicRecorder.Code, http.StatusNoContent)
	}

	unauthorizedRecorder := httptest.NewRecorder()
	unauthorizedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	engine.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("protected status = %d, want %d", unauthorizedRecorder.Code, http.StatusUnauthorized)
	}

	authorizedRecorder := httptest.NewRecorder()
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	authorizedRequest.Header.Set(
		"Authorization",
		"Bearer "+signedAccessToken(t, "admin-service", "gateway", time.Now().Add(time.Hour)),
	)
	engine.ServeHTTP(authorizedRecorder, authorizedRequest)
	if authorizedRecorder.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, want %d", authorizedRecorder.Code, http.StatusOK)
	}
}
