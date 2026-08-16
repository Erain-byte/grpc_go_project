package auth

import (
	"admin/internal/config"
	"admin/pkg/apperorr"
	"fmt"
	"strings"

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
