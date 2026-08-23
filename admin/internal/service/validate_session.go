package service

import (
	"admin/internal/auth"
	"admin/internal/repository"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	authv1 "github.com/Erain-byte/grpc_go_project/proto/auth/v1"
	"gorm.io/gorm"
)

const validateSessionTimeout = 3 * time.Second

// ValidateSessionService 统一处理服务端 Session 验证规则。
// Redis Key 的格式和用户资料缓存都留在 Admin 内部，Gateway 只关心验证结果。
type ValidateSessionService struct {
	svc        *svc.ServiceContext
	adminModel repository.AdminRepository
	info       *GetAdminInfoService
}

func NewValidateSessionService(svcCtx *svc.ServiceContext) (*ValidateSessionService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil || svcCtx.Redis == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	info, err := NewGetAdminInfoService(svcCtx)
	if err != nil {
		return nil, err
	}
	return &ValidateSessionService{
		svc:        svcCtx,
		adminModel: repository.NewAdminRepository(svcCtx),
		info:       info,
	}, nil
}

func (s *ValidateSessionService) ValidateSession(
	ctx context.Context,
	req *authv1.ValidateSessionRequest,
) (*authv1.ValidateSessionResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("validate session request is required")
	}
	subjectType := strings.ToLower(strings.TrimSpace(req.GetSubjectType()))
	if subjectType != "admin" {
		return invalidSession("unsupported_subject_type"), nil
	}
	subjectID := strings.TrimSpace(req.GetSubjectId())
	adminID, err := strconv.ParseUint(subjectID, 10, strconv.IntSize)
	if err != nil || adminID == 0 {
		return invalidSession("invalid_subject_id"), nil
	}
	sessionID := strings.TrimSpace(req.GetSessionId())
	if sessionID == "" {
		return invalidSession("session_id_missing"), nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, validateSessionTimeout)
	defer cancel()
	// 登录时会创建 Session Key，退出或过期后该 Key 不再存在。
	// 因此 Redis 中是否存在该 Key，就是当前 Session 是否在线的第一层判断。
	active, err := s.svc.Redis.Exists(
		requestCtx,
		auth.SetSessionKey(sessionID, s.svc.Config.Auth),
	)
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeUnavailable, "check admin session", http.StatusServiceUnavailable)
	}
	if !active {
		return invalidSession("session_not_found"), nil
	}

	// Session 有效后再取得服务端保存的管理员角色。
	// 优先读取资料缓存；缓存未命中时查询 MySQL 并回填 Redis。
	profileKey := adminProfileRedisKey(subjectID)
	profile, hit := s.info.profileFromCache(requestCtx, profileKey)
	if !hit {
		admin, findErr := s.adminModel.FindByID(requestCtx, uint(adminID))
		if errors.Is(findErr, gorm.ErrRecordNotFound) || admin == nil {
			return invalidSession("subject_not_found"), nil
		}
		if findErr != nil {
			return nil, apperorr.Wrap(findErr, apperorr.CodeInternal, "load admin identity", http.StatusInternalServerError)
		}
		if !admin.IsEnabled() {
			return invalidSession("subject_disabled"), nil
		}
		roleCode, roleValue, roleErr := selectLoginRoleValues(admin.Roles)
		if roleErr != nil {
			return invalidSession("role_unavailable"), nil
		}
		profile = cachedAdminProfile{
			ID:       subjectID,
			Username: admin.Username,
			Email:    stringValue(admin.Email),
			Role:     roleValue,
			RoleCode: roleCode,
		}
		s.info.cacheProfile(requestCtx, profileKey, profile)
	}
	if profile.ID != subjectID || strings.TrimSpace(profile.RoleCode) == "" {
		return invalidSession("identity_mismatch"), nil
	}

	return &authv1.ValidateSessionResponse{
		// Gateway 只应在 Valid=true 时采用下面返回的可信身份字段。
		Valid:       true,
		SubjectType: subjectType,
		SubjectId:   profile.ID,
		Role:        profile.RoleCode,
		SessionId:   sessionID,
	}, nil
}

func invalidSession(reason string) *authv1.ValidateSessionResponse {
	// Session 无效属于正常的认证结果，不是服务异常，因此返回响应而不是 error。
	return &authv1.ValidateSessionResponse{Valid: false, Reason: reason}
}
