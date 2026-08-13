package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"gateway/internal/config"
	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserID    = "auth.user_id"
	ContextRole      = "auth.role"
	ContextSessionID = "auth.session_id"
	ContextClaims    = "auth.claims"
)

// AccessTokenClaims contains the identity propagated by an access JWT.
type AccessTokenClaims struct {
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

type JWTMiddleware struct {
	secret         []byte
	algorithm      string
	audience       string
	allowedIssuers map[string]struct{}
}

func NewJWTMiddleware(cfg config.AuthConfig) (*JWTMiddleware, error) {
	accessConfig := cfg.AccessToken
	if len(accessConfig.Secret) < 32 {
		return nil, fmt.Errorf("access token secret must contain at least 32 bytes")
	}
	if accessConfig.Algorithm != jwt.SigningMethodHS256.Alg() {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", accessConfig.Algorithm)
	}
	if strings.TrimSpace(accessConfig.Audience) == "" {
		return nil, fmt.Errorf("access token audience is empty")
	}

	allowedIssuers := make(map[string]struct{}, len(accessConfig.Issuers))
	for _, issuer := range accessConfig.Issuers {
		issuer = strings.TrimSpace(issuer)
		if issuer != "" {
			allowedIssuers[issuer] = struct{}{}
		}
	}
	if len(allowedIssuers) == 0 {
		return nil, fmt.Errorf("access token issuers are empty")
	}

	return &JWTMiddleware{
		secret:         []byte(accessConfig.Secret),
		algorithm:      accessConfig.Algorithm,
		audience:       accessConfig.Audience,
		allowedIssuers: allowedIssuers,
	}, nil
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

// ValidateJWT verifies the signature and registered claims, then checks that
// the issuer belongs to one of the configured identity services.
func (m *JWTMiddleware) ValidateJWT(tokenString string) (*AccessTokenClaims, error) {
	claims := new(AccessTokenClaims)
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != m.algorithm {
				return nil, fmt.Errorf("unexpected JWT algorithm %q", token.Method.Alg())
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{m.algorithm}),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, apperror.Wrap(
			err,
			apperror.CodeUnauthorized,
			"invalid or expired access token",
			http.StatusUnauthorized,
		)
	}
	if !token.Valid {
		return nil, apperror.Unauthorized("invalid access token")
	}
	if claims.Subject == "" {
		return nil, apperror.Unauthorized("access token subject is empty")
	}
	if claims.SessionID == "" {
		return nil, apperror.Unauthorized("access token session is empty")
	}
	if _, ok := m.allowedIssuers[claims.Issuer]; !ok {
		return nil, apperror.Unauthorized("access token issuer is not allowed")
	}
	return claims, nil
}

// Handle authenticates only the route groups where it is explicitly added.
// Public routes do not register this handler and therefore need no whitelist.
func (m *JWTMiddleware) Handle(c *gin.Context) {
	tokenString, err := extractBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		Fail(c, err)
		return
	}

	claims, err := m.ValidateJWT(tokenString)
	if err != nil {
		Fail(c, err)
		return
	}

	c.Set(ContextUserID, claims.Subject)
	c.Set(ContextRole, claims.Role)
	c.Set(ContextSessionID, claims.SessionID)
	c.Set(ContextClaims, claims)
	c.Next()
}
