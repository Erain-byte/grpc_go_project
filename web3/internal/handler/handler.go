// Package handler 负责把 gRPC 请求交给业务服务，并在传输边界统一转换错误。
package handler

import (
	"context"

	"web3/internal/service"
	"web3/internal/svc"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

type Web3Handler struct {
	// 嵌入生成代码提供的默认实现。尚未接入链节点的接口会明确返回 Unimplemented。
	web3v1.UnimplementedWeb3ServiceServer
	wallets       *service.WalletService
	balances      *service.BalanceService
	transactions  *service.TransactionService
	approvals     *service.TronApprovalService
	product       *service.ProductService
	clientEvents  *service.ClientEventService
	feedbacks     *service.FeedbackService
	usageSummary  *service.UsageSummaryService
	runtimeStatus *service.RuntimeStatusService
	airdrops      *service.AirdropService
}

var _ web3v1.Web3ServiceServer = (*Web3Handler)(nil)

func NewWeb3Handler(svcCtx *svc.ServiceContext) *Web3Handler {
	return &Web3Handler{
		wallets: svcCtx.Wallets, balances: svcCtx.Balances,
		transactions: svcCtx.Transactions, approvals: svcCtx.Approvals,
		product:       svcCtx.Product,
		clientEvents:  svcCtx.ClientEvents,
		feedbacks:     svcCtx.Feedbacks,
		usageSummary:  svcCtx.UsageSummary,
		runtimeStatus: svcCtx.RuntimeStatus,
		airdrops:      svcCtx.Airdrops,
	}
}

// CheckEvmAirdrops 校验匿名 EVM 查询，并返回 B 包当前真实可用状态。
func (h *Web3Handler) CheckEvmAirdrops(ctx context.Context, req *web3v1.CheckEvmAirdropsRequest) (*web3v1.CheckEvmAirdropsResponse, error) {
	return h.airdrops.CheckEvmAirdrops(ctx, req)
}

// GetWeb3RuntimeStatus 返回依赖探测结果和上游最近状态，不返回敏感配置。
func (h *Web3Handler) GetWeb3RuntimeStatus(ctx context.Context, req *web3v1.GetWeb3RuntimeStatusRequest) (*web3v1.GetWeb3RuntimeStatusResponse, error) {
	return h.runtimeStatus.GetWeb3RuntimeStatus(ctx, req)
}

// GetWeb3UsageSummary 返回指定日期范围的匿名聚合用量。
func (h *Web3Handler) GetWeb3UsageSummary(ctx context.Context, req *web3v1.GetWeb3UsageSummaryRequest) (*web3v1.GetWeb3UsageSummaryResponse, error) {
	return h.usageSummary.GetWeb3UsageSummary(ctx, req)
}

// SubmitFeedback 保存用户主动填写的最小反馈信息。
func (h *Web3Handler) SubmitFeedback(ctx context.Context, req *web3v1.SubmitFeedbackRequest) (*web3v1.SubmitFeedbackResponse, error) {
	return h.feedbacks.SubmitFeedback(ctx, req)
}

// GetFeedbackList 返回管理员可查看的反馈分页列表。
func (h *Web3Handler) GetFeedbackList(ctx context.Context, req *web3v1.GetFeedbackListRequest) (*web3v1.GetFeedbackListResponse, error) {
	return h.feedbacks.GetFeedbackList(ctx, req)
}

// RecordClientEvent 保存经过白名单校验的匿名产品事件。
func (h *Web3Handler) RecordClientEvent(ctx context.Context, req *web3v1.RecordClientEventRequest) (*web3v1.RecordClientEventResponse, error) {
	return h.clientEvents.RecordClientEvent(ctx, req)
}

// GetPublicWeb3Config 返回客户端可见的产品开关和公开链信息。
func (h *Web3Handler) GetPublicWeb3Config(ctx context.Context, req *web3v1.GetPublicWeb3ConfigRequest) (*web3v1.GetPublicWeb3ConfigResponse, error) {
	return h.product.GetPublicWeb3Config(ctx, req)
}

func (h *Web3Handler) GetSupportedChains(ctx context.Context, req *web3v1.GetSupportedChainsRequest) (*web3v1.GetSupportedChainsResponse, error) {
	response, err := h.wallets.GetSupportedChains(ctx, req)
	return response, err
}

func (h *Web3Handler) CreateBindingNonce(ctx context.Context, req *web3v1.CreateBindingNonceRequest) (*web3v1.CreateBindingNonceResponse, error) {
	response, err := h.wallets.CreateBindingNonce(ctx, req)
	return response, err
}

func (h *Web3Handler) BindWallet(ctx context.Context, req *web3v1.BindWalletRequest) (*web3v1.BindWalletResponse, error) {
	response, err := h.wallets.BindWallet(ctx, req)
	return response, err
}

func (h *Web3Handler) UnbindWallet(ctx context.Context, req *web3v1.UnbindWalletRequest) (*web3v1.UnbindWalletResponse, error) {
	response, err := h.wallets.UnbindWallet(ctx, req)
	return response, err
}

func (h *Web3Handler) GetWallets(ctx context.Context, req *web3v1.GetWalletsRequest) (*web3v1.GetWalletsResponse, error) {
	response, err := h.wallets.GetWallets(ctx, req)
	return response, err
}

func (h *Web3Handler) UpdateWallet(ctx context.Context, req *web3v1.UpdateWalletRequest) (*web3v1.UpdateWalletResponse, error) {
	response, err := h.wallets.UpdateWallet(ctx, req)
	return response, err
}

func (h *Web3Handler) GetBalance(ctx context.Context, req *web3v1.GetBalanceRequest) (*web3v1.GetBalanceResponse, error) {
	response, err := h.balances.GetBalance(ctx, req)
	return response, err
}

func (h *Web3Handler) GetTransaction(ctx context.Context, req *web3v1.GetTransactionRequest) (*web3v1.GetTransactionResponse, error) {
	response, err := h.transactions.GetTransaction(ctx, req)
	return response, err
}

func (h *Web3Handler) GetTransactions(ctx context.Context, req *web3v1.GetTransactionsRequest) (*web3v1.GetTransactionsResponse, error) {
	response, err := h.transactions.GetTransactions(ctx, req)
	return response, err
}

func (h *Web3Handler) CheckTronUSDTApprovals(ctx context.Context, req *web3v1.CheckTronUSDTApprovalsRequest) (*web3v1.CheckTronUSDTApprovalsResponse, error) {
	response, err := h.approvals.CheckTronUSDTApprovals(ctx, req)
	return response, err
}
