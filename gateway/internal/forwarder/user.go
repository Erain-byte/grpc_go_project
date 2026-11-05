package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/pkg/apperror"
	"io"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	pbWeb3 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

// user 管理服务转发器
// UserForwarder 将 Gateway 的用户相关请求转发给 UserService。
//
// Web3 请求也会经过该转发器，但这里取得的是 UserServiceClient。UserService
// 再负责转发到 Web3Service，因此 Gateway 不直接调用 Web3Service。
type UserForwarder struct {
	pbUser.UnimplementedUserServiceServer
	base *BaseForwarder[pbUser.UserServiceClient]
}

// 构造函数
// NewUserForwarder 创建面向 user-service 的 gRPC 转发器。
func NewUserForwarder(clinMgnager *clien.ClientManager) *UserForwarder {
	return &UserForwarder{
		base: NewBaseForwarder(clinMgnager, "user-service", pbUser.NewUserServiceClient),
	}

}

func (u *UserForwarder) GetUserInfo(ctx context.Context, req *pbUser.GetUserInfoRequest) (*pbUser.GetUserInfoResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetUserInfo(ctx, req)
}

func (u *UserForwarder) GetUserList(ctx context.Context, req *pbUser.GetUserListRequest) (*pbUser.GetUserListResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)

	}
	return client.GetUserList(ctx, req)
}

func (u *UserForwarder) CreateUser(ctx context.Context, req *pbUser.CreateUserRequest) (*pbUser.CreateUserResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)

	}
	return client.CreateUser(ctx, req)
}

func (u *UserForwarder) Chat(ctx context.Context, req *pbLlm.ChatRequest) (*pbLlm.ChatResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.Chat(ctx, req)
}

// OpenStreamChat opens the UserService stream used by the HTTP-to-SSE adapter.
func (u *UserForwarder) OpenStreamChat(ctx context.Context, req *pbLlm.StreamChatRequest) (pbUser.UserService_StreamChatClient, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.StreamChat(ctx, req)
}

func (u *UserForwarder) StreamChat(req *pbLlm.StreamChatRequest, stream pbUser.UserService_StreamChatServer) error {
	if req == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	if stream == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream is nil"))
	}
	clientStream, err := u.OpenStreamChat(stream.Context(), req)
	if err != nil {
		return err
	}
	for {
		resp, recvErr := clientStream.Recv()
		if recvErr == io.EOF {
			return nil
		}
		if recvErr != nil {
			return apperror.ToGRPC(recvErr)
		}
		if sendErr := stream.Send(resp); sendErr != nil {
			return apperror.ToGRPC(sendErr)
		}
		if resp.GetFinished() {
			return nil
		}
	}
}

// 注册接口InitializeUser
func (u *UserForwarder) InitializeUser(ctx context.Context, req *pbUser.InitializeUserRequest) (*pbUser.InitializeUserResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.InitializeUser(ctx, req)
}

func (u *UserForwarder) GetSupportedChains(ctx context.Context, req *pbWeb3.GetSupportedChainsRequest) (*pbWeb3.GetSupportedChainsResponse, error) {
	// client 的类型是 UserServiceClient，而不是 Web3ServiceClient。
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetSupportedChains(ctx, req)
}

func (u *UserForwarder) CreateBindingNonce(ctx context.Context, req *pbWeb3.CreateBindingNonceRequest) (*pbWeb3.CreateBindingNonceResponse, error) {
	// UserService 会继续把当前请求的认证元数据转发给 Web3Service。
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.CreateBindingNonce(ctx, req)
}

func (u *UserForwarder) BindWallet(ctx context.Context, req *pbWeb3.BindWalletRequest) (*pbWeb3.BindWalletResponse, error) {
	// Gateway 只做入口转发；钱包签名校验由 Web3Service 处理。
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.BindWallet(ctx, req)
}

func (u *UserForwarder) UnbindWallet(ctx context.Context, req *pbWeb3.UnbindWalletRequest) (*pbWeb3.UnbindWalletResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.UnbindWallet(ctx, req)
}

func (u *UserForwarder) GetWallets(ctx context.Context, req *pbWeb3.GetWalletsRequest) (*pbWeb3.GetWalletsResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetWallets(ctx, req)
}

func (u *UserForwarder) UpdateWallet(ctx context.Context, req *pbWeb3.UpdateWalletRequest) (*pbWeb3.UpdateWalletResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.UpdateWallet(ctx, req)
}

func (u *UserForwarder) GetBalance(ctx context.Context, req *pbWeb3.GetBalanceRequest) (*pbWeb3.GetBalanceResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetBalance(ctx, req)
}

func (u *UserForwarder) GetTransaction(ctx context.Context, req *pbWeb3.GetTransactionRequest) (*pbWeb3.GetTransactionResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetTransaction(ctx, req)
}

func (u *UserForwarder) GetTransactions(ctx context.Context, req *pbWeb3.GetTransactionsRequest) (*pbWeb3.GetTransactionsResponse, error) {
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetTransactions(ctx, req)
}

// CheckTronUSDTApprovals 将公开的 TRON USDT 授权查询转发给 UserService。
// UserService 会继续将请求转发给 Web3Service，Gateway 不直接访问 Web3Service。
func (u *UserForwarder) CheckTronUSDTApprovals(
	ctx context.Context,
	req *pbWeb3.CheckTronUSDTApprovalsRequest,
) (*pbWeb3.CheckTronUSDTApprovalsResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := u.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.CheckTronUSDTApprovals(ctx, req)
}
