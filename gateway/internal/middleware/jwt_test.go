package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gateway/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type jwtFixture struct {
	middleware *JWTMiddleware
	privateKey *rsa.PrivateKey
	issuer     string
}

func newJWTFixture(t *testing.T) *jwtFixture {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/certs" {
			http.NotFound(w, r)
			return
		}
		e := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(e),
		}}})
	}))
	t.Cleanup(server.Close)
	issuer := server.URL + "/realms/grpc-project"
	middleware, err := NewJWTMiddleware(config.AuthConfig{AccessToken: config.AccessTokenConfig{
		Issuers: []string{issuer}, Audience: "grpc-api", Algorithm: "RS256", JWKSURL: server.URL + "/certs",
		RolePriority: []string{"super-admin", "admin", "user"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return &jwtFixture{middleware: middleware, privateKey: privateKey, issuer: issuer}
}

func (f *jwtFixture) token(t *testing.T, issuer, audience string, expiresAt time.Time) string {
	t.Helper()
	claims := AccessTokenClaims{
		SessionID: "session-1", PreferredUsername: "admin-test",
		RealmAccess: RealmAccess{Roles: []string{"offline_access", "admin"}},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: issuer, Subject: "keycloak-user-id", ID: "token-1",
			Audience: jwt.ClaimStrings{audience}, ExpiresAt: jwt.NewNumericDate(expiresAt), IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(f.privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestExtractBearerToken(t *testing.T) {
	got, err := extractBearerToken("Bearer token-value")
	if err != nil || got != "token-value" {
		t.Fatalf("token = %q, error = %v", got, err)
	}
}

func TestExtractRequestTokenFromOAuth2ProxyHeader(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set(forwardedAccessTokenHeader, "proxy-access-token")
	got, err := extractRequestToken(c)
	if err != nil || got != "proxy-access-token" {
		t.Fatalf("token = %q, error = %v", got, err)
	}
}

func TestValidateJWT(t *testing.T) {
	f := newJWTFixture(t)
	claims, err := f.middleware.ValidateJWT(f.token(t, f.issuer, "grpc-api", time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "keycloak-user-id" || claims.PrimaryRole(f.middleware.rolePriority) != "admin" || claims.SessionID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestValidateJWTRejectsUnknownIssuer(t *testing.T) {
	f := newJWTFixture(t)
	if _, err := f.middleware.ValidateJWT(f.token(t, "https://unknown.example", "grpc-api", time.Now().Add(time.Hour))); err == nil {
		t.Fatal("unknown issuer was accepted")
	}
}

func TestJWTMiddlewareOnlyProtectsConfiguredRouteGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	f := newJWTFixture(t)
	engine := gin.New()
	engine.Use(ErrorHandler(nil))
	engine.GET("/public", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	protected := engine.Group("/protected", f.middleware.Handle)
	protected.GET("", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"user_id": c.GetString(ContextUserID)}) })

	public := httptest.NewRecorder()
	engine.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/public", nil))
	if public.Code != http.StatusNoContent {
		t.Fatalf("public status = %d", public.Code)
	}
	unauthorized := httptest.NewRecorder()
	engine.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+f.token(t, f.issuer, "grpc-api", time.Now().Add(time.Hour)))
	engine.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d", authorized.Code)
	}
}
