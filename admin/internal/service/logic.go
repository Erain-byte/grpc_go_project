package service

import (
	"admin/internal/auth"
	"admin/internal/model"
	"admin/internal/repository"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	loginTimeout = 3 * time.Second
	//defaultSessionKeyPrefix = "admin:session:"
	adminProfileKeyPrefix = "admin:profile:"
	tokenTypeBearer       = "Bearer"
	roleCodeAdmin         = "admin"
	roleCodeSuperAdmin    = "super_admin"
)

type LogicService struct {
	svc          *svc.ServiceContext
	auth         *auth.TokenManager
	adminModel   repository.AdminRepository
	adminSession repository.SessionRepository
	roleModel    repository.RoleRepository
}

type cachedAdminProfile struct {
	ID       string            `json:"id"`
	Username string            `json:"username"`
	Email    string            `json:"email"`
	Role     adminv1.AdminRole `json:"role"`
	RoleCode string            `json:"role_code"`
}

func NewLogicService(svcCtx *svc.ServiceContext) (*LogicService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil || svcCtx.Redis == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	tokenManager, err := auth.NewTokenManager(svcCtx.Config.Auth)
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeInternal, "initialize token manager", http.StatusInternalServerError)
	}
	return &LogicService{
		svc:          svcCtx,
		auth:         tokenManager,
		adminModel:   repository.NewAdminRepository(svcCtx),
		adminSession: repository.NewAdminSessionRepository(svcCtx),
		roleModel:    repository.NewRoleRepository(svcCtx),
	}, nil
}

func (l *LogicService) Login(ctx context.Context, req *adminv1.LoginRequest) (*adminv1.LoginResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("login request is required")
	}
	username := strings.TrimSpace(req.GetUsername())
	if username == "" || req.GetPassword() == "" {
		return nil, apperorr.InvalidArgument("username and password are required")
	}

	loginCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	admin, err := l.adminModel.FindByUsername(loginCtx, username)
	if err != nil || admin == nil {
		return nil, apperorr.Unauthorized("username or password is incorrect")
	}
	if !admin.IsEnabled() {
		return nil, apperorr.Forbidden("admin account is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.GetPassword())); err != nil {
		return nil, apperorr.Unauthorized("username or password is incorrect")
	}

	roles, err := l.roleModel.FindByAdminID(loginCtx, admin.ID)
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeInternal, "query admin roles", http.StatusInternalServerError)
	}
	roleCode, roleValue, err := selectLoginRole(roles)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	sessionID := uuid.NewString()
	accessToken, err := l.auth.GenerateAccessToken(strconv.FormatUint(uint64(admin.ID), 10), roleCode, sessionID, now)
	if err != nil {
		return nil, err
	}
	refreshToken, refreshTokenHash, err := l.auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	session := &model.AdminSessionModel{
		AdminID:          admin.ID,
		SessionID:        sessionID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        now.Add(l.auth.RefreshTTL()),
		Status:           model.SessionStatusActive,
	}
	if err := l.adminSession.Create(loginCtx, session); err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeInternal, "create admin session", http.StatusInternalServerError)
	}

	//redisKey := l.sessionRedisKey(sessionID)
	redisKey := auth.SetSessionKey(sessionID, l.svc.Config.Auth)
	if err := l.svc.Redis.Set(loginCtx, redisKey, refreshTokenHash, l.auth.RefreshTTL()); err != nil {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), loginTimeout)
		defer rollbackCancel()
		if revokeErr := l.adminSession.Revoke(rollbackCtx, sessionID, now); revokeErr != nil && l.svc.Logger != nil {
			l.svc.Logger.Error("rollback admin session after Redis failure", zap.Error(revokeErr))
		}
		return nil, apperorr.Wrap(err, apperorr.CodeUnavailable, "cache admin session", http.StatusServiceUnavailable)
	}

	adminID := strconv.FormatUint(uint64(admin.ID), 10)
	profile := cachedAdminProfile{
		ID:       adminID,
		Username: admin.Username,
		Email:    stringValue(admin.Email),
		Role:     roleValue,
		RoleCode: roleCode,
	}
	if profileData, marshalErr := json.Marshal(profile); marshalErr != nil {
		if l.svc.Logger != nil {
			l.svc.Logger.Warn("encode admin profile cache", zap.Error(marshalErr))
		}
	} else if cacheErr := l.svc.Redis.Set(loginCtx, adminProfileRedisKey(adminID), profileData, l.auth.AccessTTL()); cacheErr != nil && l.svc.Logger != nil {
		// 用户资料缓存可以从数据库重建，因此写入失败不应该让一次合法登录失败。
		l.svc.Logger.Warn("cache admin profile", zap.Error(cacheErr))
	}

	return &adminv1.LoginResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        tokenTypeBearer,
		AccessExpiresIn:  int64(l.auth.AccessTTL().Seconds()),
		RefreshExpiresIn: int64(l.auth.RefreshTTL().Seconds()),
		SessionId:        sessionID,
		Admin: &adminv1.Admin{
			Id:       adminID,
			Username: admin.Username,
			Email:    stringValue(admin.Email),
			Role:     roleValue,
		},
	}, nil
}

func adminProfileRedisKey(adminID string) string {
	return adminProfileKeyPrefix + adminID
}

/*func (l *LogicService) sessionRedisKey(sessionID string) string {
	prefix := strings.TrimSpace(l.svc.Config.Auth.RefreshToken.RedisKeyPrefix)
	if prefix == "" {
		prefix = defaultSessionKeyPrefix
	}
	return fmt.Sprintf("%s%s", prefix, sessionID)
}*/

func selectLoginRole(roles []*model.RoleModel) (string, adminv1.AdminRole, error) {
	for _, role := range roles {
		if role != nil && role.Status == 1 && strings.EqualFold(strings.TrimSpace(role.Code), roleCodeSuperAdmin) {
			return roleCodeSuperAdmin, adminv1.AdminRole_ADMIN_ROLE_SUPER_ADMIN, nil
		}
	}
	for _, role := range roles {
		if role != nil && role.Status == 1 && strings.EqualFold(strings.TrimSpace(role.Code), roleCodeAdmin) {
			return roleCodeAdmin, adminv1.AdminRole_ADMIN_ROLE_ADMIN, nil
		}
	}
	return "", adminv1.AdminRole_ADMIN_ROLE_UNSPECIFIED, apperorr.Forbidden("admin has no available role")
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
