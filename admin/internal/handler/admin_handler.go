package handler

import (
	"admin/internal/service"
	"admin/internal/svc"
	"context"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

type AdminHandler struct {
	adminv1.UnimplementedAdminServiceServer
	logic  *service.LogicService
	logout *service.LogoutService
	info   *service.GetAdminInfoService
}

func NewAdminHandler(svcCtx *svc.ServiceContext) (*AdminHandler, error) {
	logic, err := service.NewLogicService(svcCtx)
	if err != nil {
		return nil, err
	}
	logout, err := service.NewLogoutReq(svcCtx)
	if err != nil {
		return nil, err
	}
	info, err := service.NewGetAdminInfoService(svcCtx)
	if err != nil {
		return nil, err
	}
	return &AdminHandler{
		logic:  logic,
		logout: logout,
		info:   info,
	}, nil
}

func (h *AdminHandler) Login(ctx context.Context, req *adminv1.LoginRequest) (*adminv1.LoginResponse, error) {
	return h.logic.Login(ctx, req)
}

func (h *AdminHandler) Logout(ctx context.Context, req *adminv1.LogoutRequest) (*adminv1.LogoutResponse, error) {
	return h.logout.Logout(ctx, req)
}

func (h *AdminHandler) GetAdminInfo(ctx context.Context, req *adminv1.GetAdminInfoRequest) (*adminv1.GetAdminInfoResponse, error) {
	return h.info.GetAdminInfo(ctx, req)
}
