package auth

import "context"

type AuthInfo struct {
	AdminID   string
	Role      string
	SessionID string
	TokenID   string
}
type contextKey struct{}

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
