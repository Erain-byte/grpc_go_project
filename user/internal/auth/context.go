package auth

import "context"

type Identity struct {
	//主体唯一 ID
	Subject  string
	Username string
	Email    string
	Role     string
	//会话ID
	SessionID string
	TokenID   string
}
type contextKey struct{}

// 写入上下文
func NewContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

//读取上下文
func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	if !ok {
		return Identity{}, ok
	}
	return identity, ok
}
