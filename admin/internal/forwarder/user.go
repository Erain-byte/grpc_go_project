package forwarder

import (
	clien "admin/internal/grpcclient"
	apperror "admin/pkg/apperorr"
	"context"

	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

// UserForwarder 从user服务获取数据
type UserForwarder struct {
	pbUser.UnimplementedUserServiceServer
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
	clien, err := u.base.Forward(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.GetUserInfo(ctx, req)
}

// GetUserList
func (u *UserForwarder) GetUserList(ctx context.Context, req *pbUser.GetUserListRequest) (*pbUser.GetUserListResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty req"))
	}
	clien, err := u.base.Forward(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.GetUserList(ctx, req)
}
