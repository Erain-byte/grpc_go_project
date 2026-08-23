package handler

import (
	"admin/internal/service"
	"admin/internal/svc"
	"context"

	authv1 "github.com/Erain-byte/grpc_go_project/proto/auth/v1"
)

// AuthHandler 是 AuthService 的 gRPC 入口，只负责把请求转交给业务层。
// 当前认证服务由 Admin 进程承载，将来可以迁移到独立 Auth 进程。
type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	validation *service.ValidateSessionService
}

func NewAuthHandler(svcCtx *svc.ServiceContext) (*AuthHandler, error) {
	validation, err := service.NewValidateSessionService(svcCtx)
	if err != nil {
		return nil, err
	}
	return &AuthHandler{validation: validation}, nil
}

func (h *AuthHandler) ValidateSession(
	ctx context.Context,
	req *authv1.ValidateSessionRequest,
) (*authv1.ValidateSessionResponse, error) {
	// Handler 不直接读取 Redis，具体验证规则由 ValidateSessionService 负责。
	return h.validation.ValidateSession(ctx, req)
}
