package forwarder

import (
	clien "admin/internal/grpcclient"
	apperror "admin/pkg/apperorr"
	"context"

	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	pbWeb3 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

// UserForwarder 从user服务获取数据
type UserForwarder struct {
	//pbUser.UnimplementedUserServiceServer
	base *BaseForwarder[pbUser.UserServiceClient]
}

func NewUserForwarder(ClientManager *clien.ClientManager) *UserForwarder {
	return &UserForwarder{
		base: NewBaseForwarder(ClientManager, "user-service", pbUser.NewUserServiceClient),
	}

}

// GetUserInfo获取用户信息
func (u *UserForwarder) GetUserInfo(ctx context.Context, req *pbUser.GetUserInfoRequest) (*pbUser.GetUserInfoResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.Client(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetUserInfo(OutgoingContext(ctx), req)
}

// GetUserList
func (u *UserForwarder) GetUserList(ctx context.Context, req *pbUser.GetUserListRequest) (*pbUser.GetUserListResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	client, err := u.base.Client(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetUserList(OutgoingContext(ctx), req)
}

func (u *UserForwarder) GetWeb3UsageSummary(ctx context.Context, req *pbWeb3.GetWeb3UsageSummaryRequest) (*pbWeb3.GetWeb3UsageSummaryResponse, error) {
	client, err := u.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetWeb3UsageSummary(OutgoingContext(ctx), req)
}

func (u *UserForwarder) GetWeb3RuntimeStatus(ctx context.Context, req *pbWeb3.GetWeb3RuntimeStatusRequest) (*pbWeb3.GetWeb3RuntimeStatusResponse, error) {
	client, err := u.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetWeb3RuntimeStatus(OutgoingContext(ctx), req)
}

func (u *UserForwarder) GetFeedbackList(ctx context.Context, req *pbWeb3.GetFeedbackListRequest) (*pbWeb3.GetFeedbackListResponse, error) {
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := u.client(ctx)
	if err != nil {
		return nil, err
	}
	return client.GetFeedbackList(OutgoingContext(ctx), req)
}

func (u *UserForwarder) client(ctx context.Context) (pbUser.UserServiceClient, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("context is nil"))
	}
	if u == nil || u.base == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("user forwarder is nil"))
	}
	client, err := u.base.Client(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client, nil
}
