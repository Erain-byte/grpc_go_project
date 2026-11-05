// Package auth 保存经过认证中间件验证的调用者身份。
package auth

import "context"

type Identity struct {
	Subject   string
	Username  string
	Email     string
	Role      string
	SessionID string
	TokenID   string
}

type contextKey struct{}

func NewContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}

func RequireIdentity(ctx context.Context) (Identity, bool) {
	identity, ok := FromContext(ctx)
	return identity, ok && identity.Subject != ""
}
