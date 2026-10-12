package middleware

import (
	"strings"

	"gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
)

// RequireAnyRole 要求当前用户至少拥有一个指定角色。
// 必须放在 JWTMiddleware.Handle 后执行。
func RequireAnyRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" {
			allowed[role] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		value, exists := c.Get(ContextClaims)

		if !exists {
			Fail(c, apperror.Unauthorized(
				"authentication claims are missing",
			))
			return
		}
		claims, ok := value.(*AccessTokenClaims)
		if !ok || claims == nil {
			Fail(c, apperror.Unauthorized(
				"authentication claims are invalid",
			))
			return
		}
		for _, role := range claims.RealmAccess.Roles {
			if _, ok := allowed[role]; ok {
				c.Next()
				return
			}
		}
		Fail(c, apperror.Forbidden(
			"insufficient permissions",
		))
	}
}
