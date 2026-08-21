package service

import (
	"admin/internal/auth"
	"admin/internal/repository"
	"admin/internal/svc"
	"admin/pkg/apperorr"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

const logoutTimeout = 3 * time.Second

type LogoutService struct {
	svc          *svc.ServiceContext
	sessionModel repository.SessionRepository
}

func NewLogoutReq(svcCtx *svc.ServiceContext) (*LogoutService, error) {
	if svcCtx == nil || svcCtx.Config == nil || svcCtx.DB == nil || svcCtx.Redis == nil {
		return nil, apperorr.InvalidArgument("service context dependencies are incomplete")
	}
	return &LogoutService{
		svc:          svcCtx,
		sessionModel: repository.NewAdminSessionRepository(svcCtx),
	}, nil
}

func (l *LogoutService) Logout(
	ctx context.Context,
	req *adminv1.LogoutRequest,
) (*adminv1.LogoutResponse, error) {
	if req == nil {
		return nil, apperorr.InvalidArgument("request is nil")
	}
	logoutCtx, cancel := context.WithTimeout(ctx, logoutTimeout)
	defer cancel()

	identity, ok := auth.FromContext(logoutCtx)
	if !ok {
		return nil, apperorr.Unauthorized("authentication information is missing")
	}
	sessionID := strings.TrimSpace(identity.SessionID)
	if sessionID == "" {
		return nil, apperorr.Unauthorized("session ID is missing")
	}
	parsedAdminID, err := strconv.ParseUint(identity.AdminID, 10, strconv.IntSize)
	if err != nil || parsedAdminID == 0 {
		return nil, apperorr.Unauthorized("invalid admin ID")
	}

	// Redis is the online session authority. Delete it first so the current
	// access token becomes unusable immediately, even if MySQL is unavailable.
	sessionKey := auth.SetSessionKey(sessionID, l.svc.Config.Auth)
	if err := l.svc.Redis.Delete(logoutCtx, sessionKey); err != nil {
		return nil, apperorr.Wrap(
			err,
			apperorr.CodeUnavailable,
			"delete admin session cache",
			http.StatusServiceUnavailable,
		)
	}

	if err := l.sessionModel.RevokeByAdminAndSession(
		logoutCtx,
		uint(parsedAdminID),
		sessionID,
		time.Now(),
	); err != nil {
		return nil, apperorr.Wrap(
			err,
			apperorr.CodeUnavailable,
			"persist admin session revocation",
			http.StatusServiceUnavailable,
		)
	}

	return &adminv1.LogoutResponse{
		Success: true,
		Message: "logout successful",
	}, nil
}
