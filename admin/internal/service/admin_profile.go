package service

import (
	"strings"

	"admin/internal/model"
	"admin/pkg/apperorr"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

const (
	adminProfileKeyPrefix = "admin:profile:"
	roleCodeAdmin         = "admin"
	roleCodeSuperAdmin    = "super_admin"
)

type cachedAdminProfile struct {
	ID       string            `json:"id"`
	Username string            `json:"username"`
	Email    string            `json:"email"`
	Role     adminv1.AdminRole `json:"role"`
	RoleCode string            `json:"role_code"`
}

func adminProfileRedisKey(subject string) string {
	return adminProfileKeyPrefix + subject
}

func selectLoginRoleValues(roles []model.RoleModel) (string, adminv1.AdminRole, error) {
	for i := range roles {
		if roles[i].Status == 1 && strings.EqualFold(strings.TrimSpace(roles[i].Code), roleCodeSuperAdmin) {
			return roleCodeSuperAdmin, adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN, nil
		}
	}
	for i := range roles {
		if roles[i].Status == 1 && strings.EqualFold(strings.TrimSpace(roles[i].Code), roleCodeAdmin) {
			return roleCodeAdmin, adminv1.AdminRole_ADMIN_ROLE_ADMIN, nil
		}
	}
	return "", adminv1.AdminRole_ADMIN_ROLE_UNSPECIFIED, apperorr.Forbidden("admin has no available role")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
