package service

import (
	"testing"

	"admin/internal/model"
	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

func TestNormalizeAdminPage(t *testing.T) {
	t.Parallel()
	page, pageSize, err := normalizeAdminPage(0, 0)
	if err != nil || page != 1 || pageSize != 20 {
		t.Fatalf("unexpected defaults: %d %d %v", page, pageSize, err)
	}
	if _, _, err := normalizeAdminPage(-1, 20); err == nil {
		t.Fatal("negative page must fail")
	}
	if _, _, err := normalizeAdminPage(1, 101); err == nil {
		t.Fatal("oversized page must fail")
	}
}

func TestRequestedAdminRole(t *testing.T) {
	t.Parallel()
	code, realmRole, err := requestedAdminRole(adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN)
	if err != nil || code != "super_admin" || realmRole != "super-admin" {
		t.Fatalf("unexpected role mapping: %q %q %v", code, realmRole, err)
	}
}

func TestModelAdminRolePrefersSuperAdmin(t *testing.T) {
	t.Parallel()
	roles := []model.RoleModel{{Code: "admin", Status: 1}, {Code: "super_admin", Status: 1}}
	if got := modelAdminRole(roles); got != adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN {
		t.Fatalf("unexpected role: %v", got)
	}
}
