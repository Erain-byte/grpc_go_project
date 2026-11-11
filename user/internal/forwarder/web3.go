package forwarder

import (
	"context"

	grpcclient "user/internal/grpc"
	"user/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

// Web3Forwarder 将 UserService 收到的钱包和链上查询请求转发给内部 Web3Service。
type Web3Forwarder struct {
	base *BaseForwarder[web3v1.Web3ServiceClient]
}

func NewWeb3Forwarder(clientManager *grpcclient.ClientManager) *Web3Forwarder {
	return &Web3Forwarder{
		base: NewBaseForwarder("web3-service", clientManager, web3v1.NewWeb3ServiceClient),
	}
}

func (f *Web3Forwarder) GetPublicWeb3Config(ctx context.Context, req *web3v1.GetPublicWeb3ConfigRequest) (*web3v1.GetPublicWeb3ConfigResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetPublicWeb3Config(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) RecordClientEvent(ctx context.Context, req *web3v1.RecordClientEventRequest) (*web3v1.RecordClientEventResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.RecordClientEvent(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) SubmitFeedback(ctx context.Context, req *web3v1.SubmitFeedbackRequest) (*web3v1.SubmitFeedbackResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.SubmitFeedback(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetWeb3UsageSummary(ctx context.Context, req *web3v1.GetWeb3UsageSummaryRequest) (*web3v1.GetWeb3UsageSummaryResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetWeb3UsageSummary(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetWeb3RuntimeStatus(ctx context.Context, req *web3v1.GetWeb3RuntimeStatusRequest) (*web3v1.GetWeb3RuntimeStatusResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetWeb3RuntimeStatus(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetFeedbackList(ctx context.Context, req *web3v1.GetFeedbackListRequest) (*web3v1.GetFeedbackListResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetFeedbackList(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetSupportedChains(ctx context.Context, req *web3v1.GetSupportedChainsRequest) (*web3v1.GetSupportedChainsResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetSupportedChains(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) CreateBindingNonce(ctx context.Context, req *web3v1.CreateBindingNonceRequest) (*web3v1.CreateBindingNonceResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.CreateBindingNonce(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) BindWallet(ctx context.Context, req *web3v1.BindWalletRequest) (*web3v1.BindWalletResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.BindWallet(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) UnbindWallet(ctx context.Context, req *web3v1.UnbindWalletRequest) (*web3v1.UnbindWalletResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.UnbindWallet(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetWallets(ctx context.Context, req *web3v1.GetWalletsRequest) (*web3v1.GetWalletsResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetWallets(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) UpdateWallet(ctx context.Context, req *web3v1.UpdateWalletRequest) (*web3v1.UpdateWalletResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.UpdateWallet(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetBalance(ctx context.Context, req *web3v1.GetBalanceRequest) (*web3v1.GetBalanceResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetBalance(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetTransaction(ctx context.Context, req *web3v1.GetTransactionRequest) (*web3v1.GetTransactionResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetTransaction(OutgoingContext(ctx), req)
}

func (f *Web3Forwarder) GetTransactions(ctx context.Context, req *web3v1.GetTransactionsRequest) (*web3v1.GetTransactionsResponse, error) {
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetTransactions(OutgoingContext(ctx), req)
}

// CheckTronUSDTApprovals 将公开的 TRON USDT 授权查询转发给 Web3Service。
// 该接口不依赖用户身份，但仍沿用请求上下文，以保留超时、取消和链路追踪信息。
func (f *Web3Forwarder) CheckTronUSDTApprovals(
	ctx context.Context,
	req *web3v1.CheckTronUSDTApprovalsRequest,
) (*web3v1.CheckTronUSDTApprovalsResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.CheckTronUSDTApprovals(OutgoingContext(ctx), req)
}

// CheckEvmAirdrops 将匿名 EVM 空投查询转发到 Web3Service。
func (f *Web3Forwarder) CheckEvmAirdrops(
	ctx context.Context,
	req *web3v1.CheckEvmAirdropsRequest,
) (*web3v1.CheckEvmAirdropsResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := f.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.CheckEvmAirdrops(OutgoingContext(ctx), req)
}

// client 集中检查转发调用的公共参数，避免每个方法重复访问空的 BaseForwarder。
func (f *Web3Forwarder) client(ctx context.Context) (web3v1.Web3ServiceClient, error) {
	if f == nil || f.base == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("Web3 forwarder is nil"))
	}
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	return f.base.Client(ctx)
}
