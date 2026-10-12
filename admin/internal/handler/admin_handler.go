package handler

import (
	"admin/internal/forwarder"
	"admin/internal/service"
	"admin/internal/svc"
	"context"

	adminv1 "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	llmv1 "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

type AdminHandler struct {
	adminv1.UnimplementedAdminServiceServer
	logic  *service.LogicService
	logout *service.LogoutService
	info   *service.GetAdminInfoService
	user   *forwarder.UserForwarder
	llm    *forwarder.LlmForwarder
}

var _ adminv1.AdminServiceServer = (*AdminHandler)(nil)

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
		user:   forwarder.NewUserForwarder(svcCtx.GRPCClients),
		llm:    forwarder.NewLlmForwarder(svcCtx.GRPCClients),
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

func (h *AdminHandler) GetUserInfo(ctx context.Context, req *userv1.GetUserInfoRequest) (*userv1.GetUserInfoResponse, error) {
	return h.user.GetUserInfo(ctx, req)
}

func (h *AdminHandler) GetUserList(ctx context.Context, req *userv1.GetUserListRequest) (*userv1.GetUserListResponse, error) {
	return h.user.GetUserList(ctx, req)
}

func (h *AdminHandler) Chat(ctx context.Context, req *llmv1.ChatRequest) (*llmv1.ChatResponse, error) {
	return h.llm.Chat(ctx, req)
}

func (h *AdminHandler) StreamChat(req *llmv1.StreamChatRequest, stream adminv1.AdminService_StreamChatServer) error {
	return h.llm.StreamChat(req, stream)
}
