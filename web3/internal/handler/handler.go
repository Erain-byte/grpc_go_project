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
	wallets      *service.WalletService
	balances     *service.BalanceService
	transactions *service.TransactionService
	approvals    *service.TronApprovalService
}

var _ web3v1.Web3ServiceServer = (*Web3Handler)(nil)

func NewWeb3Handler(svcCtx *svc.ServiceContext) *Web3Handler {
	return &Web3Handler{
		wallets: svcCtx.Wallets, balances: svcCtx.Balances,
		transactions: svcCtx.Transactions, approvals: svcCtx.Approvals,
	}
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
