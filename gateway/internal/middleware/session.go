package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gateway/pkg/apperror"

	authv1 "github.com/Erain-byte/grpc_go_project/proto/auth/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

const sessionValidationTimeout = 3 * time.Second

type SessionValidator interface {
	ValidateSession(context.Context, *authv1.ValidateSessionRequest, ...grpc.CallOption) (*authv1.ValidateSessionResponse, error)
}

// SessionMiddleware 负责验证 JWT 对应的服务端 Session 是否仍然有效。
// 它不直接操作 Redis，而是通过 AuthService 把存储细节留在认证服务内部。
type SessionMiddleware struct {
	validator SessionValidator
}

func NewSessionMiddleware(validator SessionValidator) (*SessionMiddleware, error) {
	if validator == nil {
		return nil, fmt.Errorf("session validator is nil")
	}
	return &SessionMiddleware{validator: validator}, nil
}

// Handle 必须放在 JWTMiddleware.Handle 后面执行，因为它需要读取已经验证过的 Claims。
func (m *SessionMiddleware) Handle(c *gin.Context) {
	// JWT 中间件使用 ContextClaims 作为键保存 *AccessTokenClaims。
	// 这里同时检查键是否存在、类型断言是否成功以及指针是否为空。
	value, exists := c.Get(ContextClaims)
	claims, ok := value.(*AccessTokenClaims)
	if !exists || !ok || claims == nil {
		Fail(c, apperror.Unauthorized("verified access token claims are missing"))
		return
	}

	// Session 验证是一次内部 gRPC 请求，使用 HTTP 请求 Context 传递取消信号，
	// 并额外限制最长等待时间，避免认证服务异常时长期占用 Gateway 请求。
	validationCtx, cancel := context.WithTimeout(c.Request.Context(), sessionValidationTimeout)
	defer cancel()
	validated, err := m.validator.ValidateSession(validationCtx, &authv1.ValidateSessionRequest{
		SubjectType: subjectTypeForIssuer(claims.Issuer),
		SubjectId:   claims.Subject,
		SessionId:   claims.SessionID,
		TokenId:     claims.ID,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	if validated == nil || !validated.GetValid() {
		Fail(c, apperror.Unauthorized("session has expired or been revoked"))
		return
	}
	if validated.GetSubjectId() == "" || validated.GetSessionId() == "" || validated.GetRole() == "" {
		Fail(c, apperror.Unauthorized("session identity is incomplete"))
		return
	}

	// 使用认证服务返回的身份覆盖 JWT 中的数据。
	// 这样角色发生变化后，Gateway 使用的是服务端确认的最新角色。
	c.Set(ContextUserID, validated.GetSubjectId())
	c.Set(ContextRole, validated.GetRole())
	c.Set(ContextSessionID, validated.GetSessionId())
	c.Next()
}

func subjectTypeForIssuer(issuer string) string {
	// 当前约定 admin-service 对应 admin、user-service 对应 user。
	issuer = strings.ToLower(strings.TrimSpace(issuer))
	return strings.TrimSuffix(issuer, "-service")
}
