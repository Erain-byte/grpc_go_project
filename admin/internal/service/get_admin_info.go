package service

import (
	"admin/internal/auth"
	"admin/internal/model"
	"admin/internal/repository"
	"admin/internal/svc"
	tracing "admin/internal/tracer"
	"admin/pkg/apperorr"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const getAdminInfoTimeout = 5 * time.Second

type GetAdminInfoService struct {
	svc        *svc.ServiceContext
	adminModel repository.AdminRepository
	tracer     trace.Tracer
}

func NewGetAdminInfoService(svcCtx *svc.ServiceContext) (*GetAdminInfoService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil || svcCtx.Redis == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	return &GetAdminInfoService{
		svc:        svcCtx,
		adminModel: repository.NewAdminRepository(svcCtx),
		tracer:     otel.Tracer("admin/internal/service"),
	}, nil
}

func (s *GetAdminInfoService) GetAdminInfo(
	ctx context.Context,
	req *adminv1.GetAdminInfoRequest,
) (_ *adminv1.GetAdminInfoResponse, returnErr error) {
	ctx, span := s.tracer.Start(
		ctx,
		"admin.get_admin_info",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "get admin information failed")
		}
		span.End()
	}()

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
	active, err := s.sessionActive(requestCtx, sessionKey)
	if err != nil {
		return nil, apperorr.Wrap(err, apperorr.CodeUnavailable, "check admin session", http.StatusServiceUnavailable)
	}
	if !active {
		return nil, apperorr.Unauthorized("session has expired or been revoked")
	}

	profileKey := adminProfileRedisKey(adminIDText)
	profile, hit, cacheErr := s.profileFromCache(requestCtx, profileKey)
	if cacheErr != nil && s.svc.Logger != nil {
		// 资料缓存可由 MySQL 重建，因此缓存读取失败只记录告警并继续查询数据库。
		s.svc.Logger.Warn("read admin profile cache", zap.Error(cacheErr))
	}
	if hit {
		return getAdminInfoResponse(profile), nil
	}

	admin, err := s.findByID(requestCtx, uint(parsedID))
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
	if cacheErr := s.cacheProfile(requestCtx, profileKey, profile); cacheErr != nil && s.svc.Logger != nil {
		// 回填失败不影响本次已经从 MySQL 获得的有效响应。
		s.svc.Logger.Warn("write admin profile cache", zap.Error(cacheErr))
	}
	return getAdminInfoResponse(profile), nil
}

// sessionActive 检查 Redis 中的在线 Session 状态。
func (s *GetAdminInfoService) sessionActive(
	ctx context.Context,
	key string,
) (active bool, returnErr error) {
	spanCtx, span := s.tracer.Start(
		ctx,
		"redis.session.exists",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "EXISTS"),
			attribute.String("cache.type", "admin_session"),
		),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "check admin session cache failed")
		}
		span.End()
	}()

	active, returnErr = s.svc.Redis.Exists(spanCtx, key)
	return active, returnErr
}

// profileFromCache 读取管理员资料缓存；未命中不是错误，Redis或JSON异常则交给调用方降级。
func (s *GetAdminInfoService) profileFromCache(
	ctx context.Context,
	key string,
) (profile cachedAdminProfile, hit bool, returnErr error) {
	spanCtx, span := s.tracer.Start(
		ctx,
		"redis.admin_profile.get",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "GET"),
			attribute.String("cache.type", "admin_profile"),
		),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "read admin profile cache failed")
		}
		span.SetAttributes(attribute.Bool("cache.hit", hit))
		span.End()
	}()

	value, returnErr := s.svc.Redis.Get(spanCtx, key)
	if returnErr != nil {
		return cachedAdminProfile{}, false, returnErr
	}
	if value == "" {
		return cachedAdminProfile{}, false, nil
	}
	if returnErr = json.Unmarshal([]byte(value), &profile); returnErr != nil {
		// 损坏缓存不阻断请求，删除后从数据库重建。
		if deleteErr := s.deleteProfileCache(spanCtx, key); deleteErr != nil && s.svc.Logger != nil {
			s.svc.Logger.Warn("delete invalid admin profile cache", zap.Error(deleteErr))
		}
		return cachedAdminProfile{}, false, returnErr
	}
	if profile.ID == "" {
		if deleteErr := s.deleteProfileCache(spanCtx, key); deleteErr != nil && s.svc.Logger != nil {
			s.svc.Logger.Warn("delete incomplete admin profile cache", zap.Error(deleteErr))
		}
		return cachedAdminProfile{}, false, nil
	}
	return profile, true, nil
}

// cacheProfile 将MySQL查询结果回填Redis；它是可重建缓存，错误由调用方记录但不阻断响应。
func (s *GetAdminInfoService) cacheProfile(
	ctx context.Context,
	key string,
	profile cachedAdminProfile,
) (returnErr error) {
	spanCtx, span := s.tracer.Start(
		ctx,
		"redis.admin_profile.set",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "SET"),
			attribute.String("cache.type", "admin_profile"),
		),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "write admin profile cache failed")
		}
		span.End()
	}()

	data, returnErr := json.Marshal(profile)
	if returnErr != nil {
		return returnErr
	}
	returnErr = s.svc.Redis.Set(
		spanCtx,
		key,
		data,
		profileCacheTTL(s.svc.Config.Auth.AccessToken.Expire),
	)
	return returnErr
}

// deleteProfileCache 删除无法解析或字段不完整的缓存数据。
func (s *GetAdminInfoService) deleteProfileCache(
	ctx context.Context,
	key string,
) (returnErr error) {
	spanCtx, span := s.tracer.Start(
		ctx,
		"redis.admin_profile.delete",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "DEL"),
			attribute.String("cache.type", "admin_profile"),
		),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "delete admin profile cache failed")
		}
		span.End()
	}()

	returnErr = s.svc.Redis.Delete(spanCtx, key)
	if returnErr != nil {
		return returnErr
	}
	return nil
}

// FindByID
func (s *GetAdminInfoService) findByID(ctx context.Context, id uint) (res *model.AdminModel, returnErr error) {
	spanCtx, span := s.tracer.Start(
		ctx,
		"admin.internal.service.findByID",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("db.system", "mysql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "admins"),
			attribute.Int64("admin.id", int64(id)),
		),
	)
	defer func() {
		if returnErr != nil {
			tracing.RecordError(span, returnErr, "find id by admin failed")
		}
		span.End()
	}()
	res, returnErr = s.adminModel.FindByID(spanCtx, id)
	if returnErr != nil {
		return nil, returnErr
	}
	if res != nil {
		span.SetAttributes(attribute.Int64("admin.id", int64(id)))
	}

	return res, returnErr
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
