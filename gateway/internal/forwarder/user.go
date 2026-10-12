package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/pkg/apperror"

	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

// user 管理服务转发器
type UserForwarder struct {
	pbUser.UnimplementedUserServiceServer
	base *BaseForwarder[pbUser.UserServiceClient]
}

// 构造函数
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
