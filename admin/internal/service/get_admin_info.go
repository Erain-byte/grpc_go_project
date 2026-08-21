package service

import (
	"admin/internal/auth"
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
	"go.uber.org/zap"
)

const getAdminInfoTimeout = 5 * time.Second

type GetAdminInfoService struct {
	svc        *svc.ServiceContext
	adminModel repository.AdminRepository
}

func NewGetAdminInfoService(svcCtx *svc.ServiceContext) (*GetAdminInfoService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil || svcCtx.Redis == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	return &GetAdminInfoService{
		svc:        svcCtx,
		adminModel: repository.NewAdminRepository(svcCtx),
	}, nil
}

func (s *GetAdminInfoService) GetAdminInfo(
	ctx context.Context,
	req *adminv1.GetAdminInfoRequest,
) (*adminv1.GetAdminInfoResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("request is nil")
	}
	requestCtx, cancel := context.WithTimeout(ctx, getAdminInfoTimeout)
	defer cancel()

	identity, ok := auth.FromContext(requestCtx)
	if !ok {
		return nil, apperorr.Unauthorized("authentication information is missing")
	}
	adminIDText := strings.TrimSpace(identity.AdminID)
	parsedID, err := strconv.ParseUint(adminIDText, 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		return nil, apperorr.Unauthorized("invalid admin ID")
	}
	if strings.TrimSpace(identity.SessionID) == "" {
		return nil, apperorr.Unauthorized("session ID is missing")
	}

	// JWT 签名正确并不代表 Session 仍然有效；退出后 Redis Key 会被删除。
	sessionKey := auth.SetSessionKey(identity.SessionID, s.svc.Config.Auth)
	active, err := s.svc.Redis.Exists(requestCtx, sessionKey)
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeUnavailable, "check admin session", http.StatusServiceUnavailable)
	}
	if !active {
		return nil, apperorr.Unauthorized("session has expired or been revoked")
	}

	profileKey := adminProfileRedisKey(adminIDText)
	profile, hit := s.profileFromCache(requestCtx, profileKey)
	if hit {
		return getAdminInfoResponse(profile), nil
	}

	admin, err := s.adminModel.FindByID(requestCtx, uint(parsedID))
	if err != nil || admin == nil {
		return nil, apperorr.Unauthorized("admin account does not exist")
	}
	if !admin.IsEnabled() {
		return nil, apperorr.Forbidden("admin account is disabled")
	}
	roleCode, roleValue, err := selectLoginRoleValues(admin.Roles)
	if err != nil {
		return nil, err
	}
	profile = cachedAdminProfile{
		ID:       adminIDText,
		Username: admin.Username,
		Email:    stringValue(admin.Email),
		Role:     roleValue,
		RoleCode: roleCode,
	}
	s.cacheProfile(requestCtx, profileKey, profile)
	return getAdminInfoResponse(profile), nil
}

func (s *GetAdminInfoService) profileFromCache(ctx context.Context, key string) (cachedAdminProfile, bool) {
	value, err := s.svc.Redis.Get(ctx, key)
	if err != nil {
		// 用户资料缓存可由 MySQL 重建，所以 Redis 读取失败时降级到数据库。
		if s.svc.Logger != nil {
			s.svc.Logger.Warn("read admin profile cache", zap.Error(err))
		}
		return cachedAdminProfile{}, false
	}
	if value == "" {
		return cachedAdminProfile{}, false
	}
	var profile cachedAdminProfile
	if err := json.Unmarshal([]byte(value), &profile); err != nil {
		if s.svc.Logger != nil {
			s.svc.Logger.Warn("decode admin profile cache", zap.Error(err))
		}
		// 损坏缓存不阻断请求，删除后从数据库重建。
		_ = s.svc.Redis.Delete(ctx, key)
		return cachedAdminProfile{}, false
	}
	if profile.ID == "" {
		_ = s.svc.Redis.Delete(ctx, key)
		return cachedAdminProfile{}, false
	}
	return profile, true
}

func (s *GetAdminInfoService) cacheProfile(ctx context.Context, key string, profile cachedAdminProfile) {
	data, err := json.Marshal(profile)
	if err != nil {
		if s.svc.Logger != nil {
			s.svc.Logger.Warn("encode admin profile cache", zap.Error(err))
		}
		return
	}
	if err := s.svc.Redis.Set(ctx, key, data, profileCacheTTL(s.svc.Config.Auth.AccessToken.Expire)); err != nil && s.svc.Logger != nil {
		s.svc.Logger.Warn("write admin profile cache", zap.Error(err))
	}
}

func profileCacheTTL(configured string) time.Duration {
	ttl, err := time.ParseDuration(configured)
	if err != nil || ttl <= 0 {
		return 15 * time.Minute
	}
	return ttl
}

func getAdminInfoResponse(profile cachedAdminProfile) *adminv1.GetAdminInfoResponse {
	return &adminv1.GetAdminInfoResponse{
		Success: true,
		Message: "get admin information successfully",
		AdminInfo: &adminv1.Admin{
			Id:       profile.ID,
			Username: profile.Username,
			Email:    profile.Email,
			Role:     profile.Role,
		},
	}
}
