package handler

import (
	"context"

	"user/internal/forwarder"

	"user/internal/service"
	"user/internal/svc"
	"user/pkg/apperror"

	llmv1 "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	users *service.UserService
	llm   *forwarder.LlmForwarder
}

var _ userv1.UserServiceServer = (*UserHandler)(nil)

func NewUserHandler(svcCtx *svc.ServiceContext) *UserHandler {
	return &UserHandler{
		users: service.NewUserService(svcCtx),
		llm:   forwarder.NewLlmForwarder(svcCtx.GRPCclient),
	}
}

func (h *UserHandler) InitializeUser(ctx context.Context, req *userv1.InitializeUserRequest) (*userv1.InitializeUserResponse, error) {
	response, err := h.users.InitializeUser(ctx, req)
	return response, apperror.ToGRPC(err)
}

func (h *UserHandler) GetUserInfo(ctx context.Context, req *userv1.GetUserInfoRequest) (*userv1.GetUserInfoResponse, error) {
	response, err := h.users.GetUserInfo(ctx, req)
	return response, apperror.ToGRPC(err)
}

// LLM的方法
func (h *UserHandler) Chat(ctx context.Context, req *llmv1.ChatRequest) (*llmv1.ChatResponse, error) {
	return h.llm.Chat(ctx, req)
}

// llm流式处理
func (h *UserHandler) StreamChat(req *llmv1.StreamChatRequest, stream userv1.UserService_StreamChatServer) error {
	return h.llm.StreamChat(req, stream)
}
