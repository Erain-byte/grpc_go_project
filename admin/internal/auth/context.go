package auth

import (
	"admin/internal/config"
	"context"
	"fmt"
	"strings"
)

type AuthInfo struct {
	AdminID   string
	Role      string
	SessionID string
	TokenID   string
}
type contextKey struct{}

const (
	defaultSessionKeyPrefix = "admin:session:"
)

func NewContext(ctx context.Context, authInfo AuthInfo) context.Context {
	return context.WithValue(ctx, contextKey{}, authInfo)
}

func FromContext(ctx context.Context) (AuthInfo, bool) {
	authInfo, ok := ctx.Value(contextKey{}).(AuthInfo)
	if !ok {
		return AuthInfo{}, false
	}
	return authInfo, ok
}

// 设置sessionKey
func SetSessionKey(sessionID string, cfg config.AuthConfig) string {
	prefix := strings.TrimSpace(cfg.RefreshToken.RedisKeyPrefix)
	if prefix == "" {
		prefix = defaultSessionKeyPrefix
	}
	return fmt.Sprintf("%s%s", prefix, sessionID)
}
