package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"web3/internal/auth"
	"web3/internal/config"
	"web3/pkg/apperror"

	commonv1 "github.com/Erain-byte/grpc_go_project/proto/common/v1"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type realmAccess struct {
	Roles []string `json:"roles"`
}

type accessTokenClaims struct {
	SessionID         string      `json:"sid"`
	PreferredUsername string      `json:"preferred_username"`
	Email             string      `json:"email"`
	RealmAccess       realmAccess `json:"realm_access"`
	jwt.RegisteredClaims
}

type AuthInterceptor struct {
	verifier     *oidc.IDTokenVerifier
	rolePriority []string
	ruleCache    sync.Map // map[string]*commonv1.AuthRule
}

func NewAuthInterceptor(cfg config.AuthConfig) (*AuthInterceptor, error) {
	access := cfg.AccessToken
	if strings.TrimSpace(access.Algorithm) != oidc.RS256 {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", access.Algorithm)
	}
	if strings.TrimSpace(access.Issuer) == "" || strings.TrimSpace(access.Audience) == "" || strings.TrimSpace(access.JWKSURL) == "" {
		return nil, fmt.Errorf("JWT issuer, audience, and JWKS URL are required")
	}
	roles := make([]string, 0, len(access.RolePriority))
	for _, role := range access.RolePriority {
		if role = normalizeRole(role); role != "" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("JWT role priority is required")
	}

	keySet := oidc.NewRemoteKeySet(context.Background(), strings.TrimSpace(access.JWKSURL))
	verifier := oidc.NewVerifier(strings.TrimSpace(access.Issuer), keySet, &oidc.Config{
		ClientID:             strings.TrimSpace(access.Audience),
		SupportedSigningAlgs: []string{oidc.RS256},
	})
	return &AuthInterceptor{verifier: verifier, rolePriority: roles}, nil
}

func (m *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		authenticatedCtx, err := m.authenticate(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(authenticatedCtx, req)
	}
}

func (m *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authenticatedCtx, err := m.authenticate(stream.Context(), info.FullMethod)
		if err != nil {
			return apperror.ToGRPC(err)
		}
		return apperror.ToGRPC(handler(srv, &authenticatedServerStream{ServerStream: stream, ctx: authenticatedCtx}))
	}
}

type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context { return s.ctx }

func (m *AuthInterceptor) authenticate(ctx context.Context, fullMethod string) (context.Context, error) {
	if strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/") {
		return ctx, nil
	}
	rule, err := m.authRuleForMethod(fullMethod)
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
	claims, err := m.validateJWT(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	role := primaryRole(claims.RealmAccess.Roles, m.rolePriority)
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

func (m *AuthInterceptor) validateJWT(ctx context.Context, rawToken string) (*accessTokenClaims, error) {
	token, err := m.verifier.Verify(ctx, rawToken)
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

func normalizeRole(role string) string {
	return strings.ReplaceAll(strings.TrimSpace(role), "_", "-")
}

func primaryRole(tokenRoles, priority []string) string {
	available := make(map[string]struct{}, len(tokenRoles))
	for _, role := range tokenRoles {
		available[normalizeRole(role)] = struct{}{}
	}
	for _, role := range priority {
		if _, ok := available[normalizeRole(role)]; ok {
			return normalizeRole(role)
		}
	}
	return ""
}

func roleAllowed(role string, allowedRoles []string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	role = normalizeRole(role)
	for _, allowed := range allowedRoles {
		if role == normalizeRole(allowed) {
			return true
		}
	}
	return false
}

func (m *AuthInterceptor) authRuleForMethod(fullMethod string) (*commonv1.AuthRule, error) {
	if cached, ok := m.ruleCache.Load(fullMethod); ok {
		return cached.(*commonv1.AuthRule), nil
	}
	rule, err := resolveAuthRule(fullMethod)
	if err != nil {
		return nil, err
	}
	actual, _ := m.ruleCache.LoadOrStore(fullMethod, rule)
	return actual.(*commonv1.AuthRule), nil
}

func resolveAuthRule(fullMethod string) (*commonv1.AuthRule, error) {
	descriptorName, err := grpcMethodDescriptorName(fullMethod)
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
		return nil, fmt.Errorf("descriptor %q has no method options", descriptorName)
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

func grpcMethodDescriptorName(fullMethod string) (protoreflect.FullName, error) {
	parts := strings.Split(strings.TrimPrefix(fullMethod, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid gRPC full method name %q", fullMethod)
	}
	return protoreflect.FullName(parts[0] + "." + parts[1]), nil
}
