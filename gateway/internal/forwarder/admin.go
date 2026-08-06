package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin"
)

// AdminForwarder 管理服务转发器
type AdminForwarder struct {
	pbAdmin.UnimplementedAdminServiceServer                                            // 管理服务接口
	base                                    *BaseForwarder[pbAdmin.AdminServiceClient] // 基础转发器
}

func NewAdminForwarder(svcCtx *svc.ServiceContext, clinMgnager *clien.ClientManager) *AdminForwarder {
	return &AdminForwarder{
		base: NewBaseForwarder[pbAdmin.AdminServiceClient](
			svcCtx,
			pbAdmin.NewAdminServiceClient,
			clinMgnager,
			"admin-service",
		),
	}
}

// log
func (a *AdminForwarder) Login(ctx context.Context, req *pbAdmin.LoginRequest) (*pbAdmin.LoginResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.Login(ctx, req)
}

// logout
func (a *AdminForwarder) Logout(ctx context.Context, req *pbAdmin.LogoutRequest) (*pbAdmin.LogoutResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.Logout(ctx, req)
}

// GetAdminInfo
func (a *AdminForwarder) GetAdminInfo(ctx context.Context, req *pbAdmin.GetAdminInfoRequest) (*pbAdmin.GetAdminInfoResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetAdminInfo(ctx, req)
}

// CreateAdmin
func (a *AdminForwarder) CreateAdmin(ctx context.Context, req *pbAdmin.CreateAdminRequest) (*pbAdmin.CreateAdminResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.CreateAdmin(ctx, req)
}

// GetAdminList
func (a *AdminForwarder) GetAdminList(ctx context.Context, req *pbAdmin.GetAdminListRequest) (*pbAdmin.GetAdminListResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetAdminList(ctx, req)
}

// RefreshToken
func (a *AdminForwarder) RefreshToken(ctx context.Context, req *pbAdmin.RefreshTokenRequest) (*pbAdmin.RefreshTokenResponse, error) {
	if ctx == nil {
		//return nil, ErrNilContext
		return nil, apperror.ToGRPC(apperror.InvalidArgument("is empty ctx"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)

	}
	return client.RefreshToken(ctx, req)
}
