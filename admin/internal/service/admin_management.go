package service

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"admin/internal/keycloak"
	"admin/internal/model"
	"admin/internal/repository"
	"admin/internal/svc"
	"admin/pkg/apperorr"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const adminManagementTimeout = 15 * time.Second

var adminUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{2,31}$`)

type AdminManagementService struct {
	admins   repository.AdminRepository
	roles    repository.RoleRepository
	identity keycloak.IdentityManager
}

func NewAdminManagementService(svcCtx *svc.ServiceContext) (*AdminManagementService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	identity, err := keycloak.NewAdminClient(svcCtx.Config.Auth.Management)
	if err != nil {
		return nil, err
	}
	return &AdminManagementService{
		admins: repository.NewAdminRepository(svcCtx), roles: repository.NewRoleRepository(svcCtx), identity: identity,
	}, nil
}

func (s *AdminManagementService) CreateAdmin(ctx context.Context, req *adminv1.CreateAdminRequest) (*adminv1.CreateAdminResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("request is nil")
	}
	username := strings.TrimSpace(req.GetUsername())
	email := strings.TrimSpace(req.GetEmail())
	password := req.GetPassword()
	roleCode, keycloakRole, err := requestedAdminRole(req.GetRole())
	if err != nil {
		return nil, err
	}
	if !adminUsernamePattern.MatchString(username) {
		return nil, apperorr.InvalidArgument("username must be 3-32 characters and contain only letters, numbers, dot, underscore or hyphen")
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email || len(email) > 254 {
		return nil, apperorr.InvalidArgument("email address is invalid")
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, apperorr.InvalidArgument("password must contain 8-128 characters")
	}

	requestCtx, cancel := context.WithTimeout(ctx, adminManagementTimeout)
	defer cancel()
	if _, findErr := s.admins.FindByUsername(requestCtx, username); findErr == nil {
		return nil, apperorr.Conflict("administrator username already exists")
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, findErr
	}
	role, err := s.roles.FindByCode(requestCtx, roleCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperorr.NotFound("administrator role is not initialized")
	}
	if err != nil {
		return nil, err
	}
	if role.Status != 1 {
		return nil, apperorr.Forbidden("administrator role is disabled")
	}

	subject, err := s.identity.CreateAdmin(requestCtx, username, email, password, keycloakRole)
	if errors.Is(err, keycloak.ErrDisabled) {
		return nil, apperorr.Unavailable("Keycloak administrator provisioning is not configured")
	}
	if errors.Is(err, keycloak.ErrConflict) {
		return nil, apperorr.Conflict("Keycloak administrator already exists")
	}
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeUnavailable, "create Keycloak administrator failed", 503)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		_ = s.identity.DeleteUser(context.WithoutCancel(ctx), subject)
		return nil, err
	}
	admin := &model.AdminModel{KeycloakSubject: &subject, Username: username, PasswordHash: string(passwordHash), Email: &email, Status: model.AdminStatusEnabled}
	if err := s.admins.CreateWithRole(requestCtx, admin, role); err != nil {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer rollbackCancel()
		_ = s.identity.DeleteUser(rollbackCtx, subject)
		return nil, err
	}
	return &adminv1.CreateAdminResponse{Success: true, Message: "administrator created", Admin: adminProto(admin)}, nil
}

func (s *AdminManagementService) GetAdminList(ctx context.Context, req *adminv1.GetAdminListRequest) (*adminv1.GetAdminListResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("request is nil")
	}
	page, pageSize, err := normalizeAdminPage(req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, adminManagementTimeout)
	defer cancel()
	admins, total, err := s.admins.List(requestCtx, int((page-1)*pageSize), int(pageSize))
	if err != nil {
		return nil, err
	}
	items := make([]*adminv1.Admin, 0, len(admins))
	for _, admin := range admins {
		items = append(items, adminProto(admin))
	}
	return &adminv1.GetAdminListResponse{Admins: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func requestedAdminRole(role adminv1.AdminRole) (string, string, error) {
	switch role {
	case adminv1.AdminRole_ADMIN_ROLE_ADMIN:
		return roleCodeAdmin, "admin", nil
	case adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN:
		return roleCodeSuperAdmin, "super-admin", nil
	default:
		return "", "", apperorr.InvalidArgument("administrator role is required")
	}
}

func normalizeAdminPage(page, pageSize int32) (int32, int32, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return 0, 0, apperorr.InvalidArgument("page must be positive and page_size must be between 1 and 100")
	}
	return page, pageSize, nil
}

func adminProto(admin *model.AdminModel) *adminv1.Admin {
	if admin == nil {
		return nil
	}
	id := strconv.FormatUint(uint64(admin.ID), 10)
	if admin.KeycloakSubject != nil && strings.TrimSpace(*admin.KeycloakSubject) != "" {
		id = strings.TrimSpace(*admin.KeycloakSubject)
	}
	return &adminv1.Admin{Id: id, Username: admin.Username, Email: stringValue(admin.Email), Role: modelAdminRole(admin.Roles)}
}

func modelAdminRole(roles []model.RoleModel) adminv1.AdminRole {
	for _, role := range roles {
		if role.Status == 1 && role.Code == roleCodeSuperAdmin {
			return adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN
		}
	}
	for _, role := range roles {
		if role.Status == 1 && role.Code == roleCodeAdmin {
			return adminv1.AdminRole_ADMIN_ROLE_ADMIN
		}
	}
	return adminv1.AdminRole_ADMIN_ROLE_UNSPECIFIED
}
