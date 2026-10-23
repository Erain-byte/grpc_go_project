package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"user/internal/auth"
	"user/internal/config"
	"user/pkg/apperror"

	commonv1 "github.com/Erain-byte/grpc_go_project/proto/common/v1"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// 本脚本验证中间件实现OIDC
// Keycloak 标准 JWT 字段：`realm_access.roles`
type realmAccess struct {
	Roles []string `json:"roles"`
}

// 自定义accessTokenClaims
type accessTokenClaims struct {
	SessionID         string      `json:"sid"`
	PreferredUsername string      `json:"preferred_username"`
	Email             string      `json:"email"`
	RealmAccess       realmAccess `json:"realm_access"`
	//嵌入，继承标准JWT字段：iss, exp, sub, iat, jti 等
	jwt.RegisteredClaims
}

// AuthInterceptor 鉴权拦截器
type AuthInterceptor struct {
	verifier     *oidc.IDTokenVerifier
	rolePriority []string
	//缓存规则
	ruleCached sync.Map
}

// 构造函数
func NewAuthInterceptor(cfg config.AuthConfig) (*AuthInterceptor, error) {
	access := cfg.AccessToken
	if strings.TrimSpace(access.Algorithm) != oidc.RS256 {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", access.Algorithm)
	}
	if strings.TrimSpace(access.Issuer) == "" || strings.TrimSpace(access.Audience) == "" {
		return nil, fmt.Errorf("JWT issuer and audience are required")
	}
	if strings.TrimSpace(access.JWKSURL) == "" {
		return nil, fmt.Errorf("JWT JWKS URL is required")
	}
	//清洗配置
	roles := make([]string, 0, len(access.RolePriority))
	for _, role := range access.RolePriority {
		if strings.TrimSpace(role) != "" {
			roles = append(roles, strings.TrimSpace(role))
		}
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("JWT role priority is required")
	}
	//创建IDTokenVerifier
	keySet := oidc.NewRemoteKeySet(context.Background(), strings.TrimSpace(access.JWKSURL))
	verifier := oidc.NewVerifier(strings.TrimSpace(access.Issuer), keySet, &oidc.Config{
		ClientID:             strings.TrimSpace(access.Audience),
		SupportedSigningAlgs: []string{oidc.RS256}, //支持签名算法
	})
	return &AuthInterceptor{
		verifier:     verifier,
		rolePriority: roles,
	}, nil
}

// Unary普通拦截器
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		authenticatedCtx, err := a.authenticate(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(authenticatedCtx, req)
	}

}

// 流式拦截器
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authenticatedCtx, err := a.authenticate(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, &authenticatedServerStream{ServerStream: stream, ctx: authenticatedCtx})
	}
}

// authenticate
func (a *AuthInterceptor) authenticate(ctx context.Context, fullMethod string) (context.Context, error) {

	if fullMethod == healthpb.Health_Check_FullMethodName {
		return ctx, nil
	}
	rule, err := a.authRuleForMethod(fullMethod)
	if err != nil {
		return nil, err
	}
	if rule.GetPublic() {
		return ctx, nil
	}
	rawToken, err := bearerTokenFromMetadata(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := a.validateJWT(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	role := primaryRole(claims.RealmAccess.Roles, a.rolePriority)
	if role == "" {
		return nil, apperror.Unauthorized("access token has no application role")
	}
	if !roleAllowed(role, rule.GetRoles()) {
		return nil, apperror.Forbidden("the current role cannot access this method")
	}
	return auth.NewContext(ctx, auth.Identity{
		Subject:   claims.Subject,
		Username:  claims.PreferredUsername,
		Email:     claims.Email,
		Role:      role,
		SessionID: claims.SessionID,
		TokenID:   claims.ID,
	}), nil
}

func (a *AuthInterceptor) validateJWT(ctx context.Context, rawToken string) (*accessTokenClaims, error) {
	token, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnauthorized, "invalid or expired access token", http.StatusUnauthorized)
	}
	claims := new(accessTokenClaims)
	if err := token.Claims(claims); err != nil {
		return nil, apperror.Wrap(err, apperror.CodeUnauthorized, "invalid access token claims", http.StatusUnauthorized)
	}
	if strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.ID) == "" {
		return nil, apperror.Unauthorized("access token subject or ID is empty")
	}
	return claims, nil
}
func bearerTokenFromMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", apperror.Unauthorized("authorization metadata is missing")
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return "", apperror.Unauthorized("authorization metadata must contain one value")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", apperror.Unauthorized("authorization metadata is invalid")
	}
	return parts[1], nil
}
func primaryRole(tokenRoles, priority []string) string {
	available := make(map[string]struct{}, len(tokenRoles))
	for _, role := range tokenRoles {
		available[strings.ReplaceAll(strings.TrimSpace(role), "_", "-")] = struct{}{}
	}
	for _, role := range priority {
		normalized := strings.ReplaceAll(strings.TrimSpace(role), "_", "-")
		if _, ok := available[normalized]; ok {
			return role
		}
	}
	return ""
}

// authRuleForMethod缓存规则
func (a *AuthInterceptor) authRuleForMethod(fullMethod string) (*commonv1.AuthRule, error) {
	if cached, ok := a.ruleCached.Load(fullMethod); ok {
		return cached.(*commonv1.AuthRule), nil
	}
	rule, err := resolveAuthRule(fullMethod)
	if err != nil {
		return nil, err
	}
	actule, _ := a.ruleCached.LoadOrStore(fullMethod, rule) //缓存规则

	return actule.(*commonv1.AuthRule), nil
}

type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context {
	return s.ctx
}
func resolveAuthRule(fullMethod string) (*commonv1.AuthRule, error) {
	descriptorName, err := grpcFullMethod(fullMethod)
	if err != nil {
		return nil, err
	}
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(descriptorName)
	if err != nil {
		return nil, err
	}
	method, ok := descriptor.(protoreflect.MethodDescriptor)
	if !ok {
		return nil, fmt.Errorf("descriptor %q is not a method", descriptorName)
	}
	options, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return nil, fmt.Errorf("descriptor %q has no options", descriptorName)
	}
	if !proto.HasExtension(options, commonv1.E_Auth) {
		return &commonv1.AuthRule{}, nil
	}
	rule, ok := proto.GetExtension(options, commonv1.E_Auth).(*commonv1.AuthRule)
	if !ok || rule == nil {
		return nil, fmt.Errorf("protobuf method %q has an invalid auth option", descriptorName)
	}
	return rule, nil
}
func grpcFullMethod(fullMethod string) (protoreflect.FullName, error) {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid grpc method %q", fullMethod)
	}
	return protoreflect.FullName(parts[0] + "." + parts[1]), nil
}

func roleAllowed(role string, roles []string) bool {
	if len(role) == 0 {
		return true
	}
	normalizedRole := strings.ReplaceAll(role, "_", "-")
	for _, allowedRole := range roles {

		if normalizedRole == allowedRole {
			return true
		}
	}
	return false
}
