package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gateway/internal/config"
	"gateway/pkg/apperror"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserID              = "auth.user_id"
	ContextUsername            = "auth.username"
	ContextRole                = "auth.role"
	ContextSessionID           = "auth.session_id"
	ContextTokenID             = "auth.token_id"
	ContextClaims              = "auth.claims"
	forwardedAccessTokenHeader = "X-Forwarded-Access-Token"
)

type RealmAccess struct {
	Roles []string `json:"roles"`
}

// AccessTokenClaims contains the Keycloak identity propagated by an access token.
type AccessTokenClaims struct {
	SessionID         string      `json:"sid"`
	PreferredUsername string      `json:"preferred_username"`
	RealmAccess       RealmAccess `json:"realm_access"`
	jwt.RegisteredClaims
}

// PrimaryRole selects the first configured application role present in the token.
func (c *AccessTokenClaims) PrimaryRole(priority []string) string {
	roles := make(map[string]struct{}, len(c.RealmAccess.Roles))
	for _, role := range c.RealmAccess.Roles {
		roles[role] = struct{}{}
	}
	for _, role := range priority {
		if _, ok := roles[role]; ok {
			return role
		}
	}
	return ""
}

type JWTMiddleware struct {
	verifier     *oidc.IDTokenVerifier
	rolePriority []string
}

func NewJWTMiddleware(cfg config.AuthConfig) (*JWTMiddleware, error) {
	accessConfig := cfg.AccessToken
	if accessConfig.Algorithm != oidc.RS256 {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", accessConfig.Algorithm)
	}
	if strings.TrimSpace(accessConfig.Audience) == "" {
		return nil, fmt.Errorf("access token audience is empty")
	}
	if len(accessConfig.Issuers) != 1 || strings.TrimSpace(accessConfig.Issuers[0]) == "" {
		return nil, fmt.Errorf("exactly one access token issuer is required")
	}
	jwksURL := strings.TrimSpace(accessConfig.JWKSURL)
	if jwksURL == "" {
		return nil, fmt.Errorf("access token JWKS URL is empty")
	}
	rolePriority := make([]string, 0, len(accessConfig.RolePriority))
	for _, role := range accessConfig.RolePriority {
		if role = strings.TrimSpace(role); role != "" {
			rolePriority = append(rolePriority, role)
		}
	}
	if len(rolePriority) == 0 {
		return nil, fmt.Errorf("access token role priority is empty")
	}

	keySet := oidc.NewRemoteKeySet(context.Background(), jwksURL)
	verifier := oidc.NewVerifier(strings.TrimSpace(accessConfig.Issuers[0]), keySet, &oidc.Config{
		ClientID:             accessConfig.Audience,
		SupportedSigningAlgs: []string{oidc.RS256},
	})
	return &JWTMiddleware{verifier: verifier, rolePriority: rolePriority}, nil
}

func extractBearerToken(authorization string) (string, error) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", apperror.Unauthorized("missing or invalid Authorization header")
	}
	if parts[1] == "" {
		return "", apperror.Unauthorized("access token is empty")
	}
	return parts[1], nil
}

func extractRequestToken(c *gin.Context) (string, error) {
	if authorization := strings.TrimSpace(c.GetHeader("Authorization")); authorization != "" {
		return extractBearerToken(authorization)
	}
	if token := strings.TrimSpace(c.GetHeader(forwardedAccessTokenHeader)); token != "" {
		return token, nil
	}
	return "", apperror.Unauthorized("access token is missing")
}

func (m *JWTMiddleware) validateJWT(ctx context.Context, tokenString string) (*AccessTokenClaims, error) {
	idToken, err := m.verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnauthorized, "invalid or expired access token", http.StatusUnauthorized)
	}
	claims := new(AccessTokenClaims)
	if err := idToken.Claims(claims); err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnauthorized, "invalid access token claims", http.StatusUnauthorized)
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, apperror.Unauthorized("access token subject is empty")
	}
	if strings.TrimSpace(claims.ID) == "" {
		return nil, apperror.Unauthorized("access token ID is empty")
	}
	if claims.PrimaryRole(m.rolePriority) == "" {
		return nil, apperror.Unauthorized("access token has no application role")
	}
	return claims, nil
}

func (m *JWTMiddleware) ValidateJWT(tokenString string) (*AccessTokenClaims, error) {
	return m.validateJWT(context.Background(), tokenString)
}

func (m *JWTMiddleware) Handle(c *gin.Context) {
	tokenString, err := extractRequestToken(c)
	if err != nil {
		Fail(c, err)
		return
	}
	claims, err := m.validateJWT(c.Request.Context(), tokenString)
	if err != nil {
		Fail(c, err)
		return
	}
	c.Set(ContextUserID, claims.Subject)
	c.Set(ContextUsername, claims.PreferredUsername)
	c.Set(ContextRole, claims.PrimaryRole(m.rolePriority))
	c.Set(ContextSessionID, claims.SessionID)
	c.Set(ContextTokenID, claims.ID)
	c.Set(ContextClaims, claims)
	c.Next()
}
