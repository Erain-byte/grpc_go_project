package handler

import (
	"context"

	"user/internal/forwarder"

	"user/internal/service"
	"user/internal/svc"
	"user/pkg/apperror"

	llmv1 "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	userv1 "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	users *service.UserService
	llm   *forwarder.LlmForwarder
	// web3 负责把钱包绑定、余额和交易查询请求转发给 Web3Service。
	web3 *forwarder.Web3Forwarder
}

var _ userv1.UserServiceServer = (*UserHandler)(nil)

func NewUserHandler(svcCtx *svc.ServiceContext) *UserHandler {
	return &UserHandler{
		users: service.NewUserService(svcCtx),
		llm:   forwarder.NewLlmForwarder(svcCtx.GRPCclient),
		// Web3Service 使用与 LLM 相同的内部 gRPC ClientManager 和 Consul 服务发现。
		web3: forwarder.NewWeb3Forwarder(svcCtx.GRPCclient),
	}
}

// -------------------- Web3 钱包与链上查询接口 --------------------

// GetSupportedChains 获取 Web3 服务当前支持的区块链网络。
func (h *UserHandler) GetSupportedChains(ctx context.Context, req *web3v1.GetSupportedChainsRequest) (*web3v1.GetSupportedChainsResponse, error) {
	return h.web3.GetSupportedChains(ctx, req)
}

// CreateBindingNonce 创建用于钱包签名验证的一次性 nonce。
func (h *UserHandler) CreateBindingNonce(ctx context.Context, req *web3v1.CreateBindingNonceRequest) (*web3v1.CreateBindingNonceResponse, error) {
	return h.web3.CreateBindingNonce(ctx, req)
}

// BindWallet 验证签名并绑定当前用户的钱包。
func (h *UserHandler) BindWallet(ctx context.Context, req *web3v1.BindWalletRequest) (*web3v1.BindWalletResponse, error) {
	return h.web3.BindWallet(ctx, req)
}

// UnbindWallet 解除当前用户的钱包绑定。
func (h *UserHandler) UnbindWallet(ctx context.Context, req *web3v1.UnbindWalletRequest) (*web3v1.UnbindWalletResponse, error) {
	return h.web3.UnbindWallet(ctx, req)
}

// GetWallets 获取当前用户已经绑定的钱包列表。
func (h *UserHandler) GetWallets(ctx context.Context, req *web3v1.GetWalletsRequest) (*web3v1.GetWalletsResponse, error) {
	return h.web3.GetWallets(ctx, req)
}

// UpdateWallet 修改钱包备注或默认钱包状态。
func (h *UserHandler) UpdateWallet(ctx context.Context, req *web3v1.UpdateWalletRequest) (*web3v1.UpdateWalletResponse, error) {
	return h.web3.UpdateWallet(ctx, req)
}

// GetBalance 查询已绑定钱包的原生币或代币余额。
func (h *UserHandler) GetBalance(ctx context.Context, req *web3v1.GetBalanceRequest) (*web3v1.GetBalanceResponse, error) {
	return h.web3.GetBalance(ctx, req)
}

// GetTransaction 查询指定链上交易的状态。
func (h *UserHandler) GetTransaction(ctx context.Context, req *web3v1.GetTransactionRequest) (*web3v1.GetTransactionResponse, error) {
	return h.web3.GetTransaction(ctx, req)
}

// GetTransactions 分页查询已绑定钱包的交易记录。
func (h *UserHandler) GetTransactions(ctx context.Context, req *web3v1.GetTransactionsRequest) (*web3v1.GetTransactionsResponse, error) {
	return h.web3.GetTransactions(ctx, req)
}

// CheckTronUSDTApprovals 接收 Gateway 的匿名查询，并转发给 Web3Service。
// 具体的 Base58Check 校验、链上查询和缓存逻辑由 Web3Service 负责。
func (h *UserHandler) CheckTronUSDTApprovals(
	ctx context.Context,
	req *web3v1.CheckTronUSDTApprovalsRequest,
) (*web3v1.CheckTronUSDTApprovalsResponse, error) {
	return h.web3.CheckTronUSDTApprovals(ctx, req)
}

// -------------------- UserService 自身业务接口 --------------------

func (h *UserHandler) InitializeUser(ctx context.Context, req *userv1.InitializeUserRequest) (*userv1.InitializeUserResponse, error) {
	response, err := h.users.InitializeUser(ctx, req)
	return response, apperror.ToGRPC(err)
}

func (h *UserHandler) GetUserInfo(ctx context.Context, req *userv1.GetUserInfoRequest) (*userv1.GetUserInfoResponse, error) {
	response, err := h.users.GetUserInfo(ctx, req)
	return response, apperror.ToGRPC(err)
}

func (h *UserHandler) GetUserList(ctx context.Context, req *userv1.GetUserListRequest) (*userv1.GetUserListResponse, error) {
	response, err := h.users.GetUserList(ctx, req)
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
