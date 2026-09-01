package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authv1 "github.com/Erain-byte/grpc_go_project/proto/auth/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// fakeAuthClient 模拟某个内部服务生成的 AuthServiceClient。
type fakeAuthClient struct {
	response *authv1.ValidateSessionResponse
	err      error
	request  *authv1.ValidateSessionRequest
}

func (f *fakeAuthClient) ValidateSession(
	_ context.Context,
	req *authv1.ValidateSessionRequest,
	_ ...grpc.CallOption,
) (*authv1.ValidateSessionResponse, error) {
	f.request = req
	return f.response, f.err
}

// fakeAuthClientProvider 记录 SessionMiddleware 根据 issuer 选择了哪个服务。
type fakeAuthClientProvider struct {
	client      authv1.AuthServiceClient
	err         error
	serviceName string
}

func (f *fakeAuthClientProvider) AuthClient(
	_ context.Context,
	serviceName string,
) (authv1.AuthServiceClient, error) {
	f.serviceName = serviceName
	return f.client, f.err
}

func newSessionTestContext(claims *AccessTokenClaims) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/protected", nil)
	if claims != nil {
		ctx.Set(ContextClaims, claims)
	}
	return ctx, recorder
}

func adminSessionClaims() *AccessTokenClaims {
	claims := &AccessTokenClaims{
		Role:      "admin",
		SessionID: "session-1",
	}
	claims.Issuer = "admin-service"
	claims.Subject = "2"
	claims.ID = "token-1"
	return claims
}

func TestSessionMiddlewareSelectsAuthServiceByIssuer(t *testing.T) {
	authClient := &fakeAuthClient{response: &authv1.ValidateSessionResponse{
		Valid:       true,
		SubjectType: "admin",
		SubjectId:   "2",
		Role:        "super_admin",
		SessionId:   "session-1",
	}}
	provider := &fakeAuthClientProvider{client: authClient}
	middleware, err := NewSessionMiddleware(provider)
	if err != nil {
		t.Fatalf("NewSessionMiddleware() error = %v", err)
	}
	ctx, _ := newSessionTestContext(adminSessionClaims())

	middleware.Handle(ctx)

	if ctx.IsAborted() {
		t.Fatal("valid session was rejected")
	}
	if provider.serviceName != "admin-service" {
		t.Fatalf("AuthClient service = %q, want admin-service", provider.serviceName)
	}
	if authClient.request == nil {
		t.Fatal("ValidateSession request was not sent")
	}
	if authClient.request.GetSubjectType() != "admin" ||
		authClient.request.GetSubjectId() != "2" ||
		authClient.request.GetSessionId() != "session-1" ||
		authClient.request.GetTokenId() != "token-1" {
		t.Fatalf("ValidateSession request = %+v", authClient.request)
	}
	// Session 服务返回的最新角色应覆盖 JWT 中的旧角色。
	if got := ctx.GetString(ContextRole); got != "super_admin" {
		t.Fatalf("context role = %q, want super_admin", got)
	}
}

func TestSessionMiddlewareRejectsMissingIssuer(t *testing.T) {
	provider := &fakeAuthClientProvider{client: &fakeAuthClient{}}
	middleware, err := NewSessionMiddleware(provider)
	if err != nil {
		t.Fatalf("NewSessionMiddleware() error = %v", err)
	}
	claims := adminSessionClaims()
	claims.Issuer = "  "
	ctx, _ := newSessionTestContext(claims)

	middleware.Handle(ctx)

	if !ctx.IsAborted() {
		t.Fatal("session without issuer was accepted")
	}
	if provider.serviceName != "" {
		t.Fatalf("AuthClient was called with service %q", provider.serviceName)
	}
}

func TestSessionMiddlewareRejectsRevokedSession(t *testing.T) {
	provider := &fakeAuthClientProvider{client: &fakeAuthClient{
		response: &authv1.ValidateSessionResponse{
			Valid:  false,
			Reason: "session_not_found",
		},
	}}
	middleware, err := NewSessionMiddleware(provider)
	if err != nil {
		t.Fatalf("NewSessionMiddleware() error = %v", err)
	}
	ctx, _ := newSessionTestContext(adminSessionClaims())

	middleware.Handle(ctx)

	if !ctx.IsAborted() {
		t.Fatal("revoked session was accepted")
	}
}

func TestSessionMiddlewareRejectsIdentityMismatch(t *testing.T) {
	provider := &fakeAuthClientProvider{client: &fakeAuthClient{
		response: &authv1.ValidateSessionResponse{
			Valid:       true,
			SubjectType: "admin",
			SubjectId:   "other-admin",
			Role:        "admin",
			SessionId:   "session-1",
		},
	}}
	middleware, err := NewSessionMiddleware(provider)
	if err != nil {
		t.Fatalf("NewSessionMiddleware() error = %v", err)
	}
	ctx, _ := newSessionTestContext(adminSessionClaims())

	middleware.Handle(ctx)

	if !ctx.IsAborted() {
		t.Fatal("mismatched session identity was accepted")
	}
}

func TestNewSessionMiddlewareRejectsNilProvider(t *testing.T) {
	if _, err := NewSessionMiddleware(nil); err == nil {
		t.Fatal("NewSessionMiddleware() error = nil, want error")
	}
}
