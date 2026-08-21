package auth

import (
	"admin/internal/config"
	"admin/pkg/apperorr"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/golang-jwt/jwt/v5"
)

type AccessToken struct {
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

type AccessTokenVer interface {
	VerifyAccessToken(tokenString string) (*AccessToken, error)
}

type JWTVerifier struct {
	secret    []byte
	algorithm string
	issuer    string
	audience  string
}

// 默认密钥前缀
func NewJWTVerifier(cfg config.AccessTokenConfig) (*JWTVerifier, error) {
	if len(cfg.Secret) == 0 {
		return nil, fmt.Errorf("secret is required")
	}
	if strings.TrimSpace(cfg.Algorithm) != jwt.SigningMethodHS256.Alg() {
		return nil, fmt.Errorf(
			"unsupported JWT algorithm %q",
			cfg.Algorithm,
		)
	}
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, fmt.Errorf("access token issuer is empty")
	}

	if strings.TrimSpace(cfg.Audience) == "" {
		return nil, fmt.Errorf("access token audience is empty")
	}

	return &JWTVerifier{
		secret:    []byte(cfg.Secret),
		algorithm: cfg.Algorithm,
		issuer:    cfg.Issuer,
		audience:  cfg.Audience,
	}, nil
}

func (v *JWTVerifier) VerifyAccessToken(
	tokenString string,
) (*AccessToken, error) {
	claims := new(AccessToken)
	//获取token
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != v.algorithm {
				return nil, apperorr.Unauthorized(
					"unexpected JWT algorithm",
				)
			}

			return v.secret, nil
		},
		//设置token的验证方式
		jwt.WithValidMethods([]string{v.algorithm}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, apperorr.Unauthorized(
			"invalid or expired access token",
		)
	}

	if !token.Valid {
		return nil, apperorr.Unauthorized(
			"invalid access token",
		)
	}

	if strings.TrimSpace(claims.Subject) == "" {
		return nil, apperorr.Unauthorized(
			"access token subject is empty",
		)
	}

	if strings.TrimSpace(claims.SessionID) == "" {
		return nil, apperorr.Unauthorized(
			"access token session is empty",
		)
	}

	return claims, nil
}

type TokenManager struct {
	secret     []byte
	algorithm  string
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(cfg config.AuthConfig) (*TokenManager, error) {
	//secretValue := strings.TrimSpace(cfg.AccessToken.Secret)
	if strings.TrimSpace(cfg.AccessToken.Secret) == "" {
		return nil, apperorr.InvalidArgument(
			"access token secret is required",
		)
	}
	algorithm := strings.TrimSpace(cfg.AccessToken.Algorithm)
	if algorithm != jwt.SigningMethodHS256.Alg() {
		return nil, apperorr.InvalidArgument(
			"unsupported access token algorithm",
		)
	}
	issuer := strings.TrimSpace(cfg.AccessToken.Issuer)

	if issuer == "" {
		return nil, apperorr.InvalidArgument(
			"access token issuer is required",
		)
	}
	audience := strings.TrimSpace(
		cfg.AccessToken.Audience,
	)
	if audience == "" {
		return nil, apperorr.InvalidArgument(
			"access token audience is required",
		)
	}

	accessTTL, err := time.ParseDuration(cfg.AccessToken.Expire)
	if err != nil || accessTTL <= 0 {
		return nil, apperorr.InvalidArgument(
			"invalid access token expire duration",
		)
	}

	refreshTTL, err := time.ParseDuration(cfg.RefreshToken.Expire)
	if err != nil || refreshTTL <= 0 {
		return nil, apperorr.InvalidArgument(
			"invalid refresh token expire duration",
		)
	}
	return &TokenManager{
		secret:     []byte(cfg.AccessToken.Secret),
		algorithm:  algorithm,
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil

}

func (m *TokenManager) GenerateAccessToken(
	adminID string,
	role string,
	sessionID string,
	now time.Time,
) (string, error) {
	adminID = strings.TrimSpace(adminID)
	role = strings.TrimSpace(role)
	sessionID = strings.TrimSpace(sessionID)
	if adminID == "" {
		return "", apperorr.InvalidArgument(
			"admin id is required",
		)
	}
	if role == "" {
		return "", apperorr.InvalidArgument(
			"role is required",
		)
	}
	if sessionID == "" {
		return "", apperorr.InvalidArgument(
			"session id is required",
		)
	}
	claims := AccessToken{
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   adminID,
			ID:        uuid.NewString(),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)
	//生成token
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", apperorr.Wrap(
			err,
			apperorr.CodeInternal,
			"failed to sign access token",
			http.StatusInternalServerError,
		)
	}
	return tokenString, nil
}

func (m *TokenManager) GenerateRefreshToken() (
	token string,
	hash string,
	err error,
) {

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", apperorr.Wrap(
			err,
			apperorr.CodeInternal,
			"failed to generate refresh token",
			http.StatusInternalServerError,
		)
	}
	token = base64.RawURLEncoding.EncodeToString(randomBytes)
	hash = HashRefreshToken(token)
	return token, hash, nil
}
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *TokenManager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *TokenManager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

// 设置sessionKey
