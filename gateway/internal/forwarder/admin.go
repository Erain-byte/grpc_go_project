package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/pkg/apperror"
	"io"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

// AdminForwarder 管理服务转发器
type AdminForwarder struct {
	pbAdmin.UnimplementedAdminServiceServer                                            // 管理服务接口
	base                                    *BaseForwarder[pbAdmin.AdminServiceClient] // 基础转发器
}

func NewAdminForwarder(clinMgnager *clien.ClientManager) *AdminForwarder {
	return &AdminForwarder{
		base: NewBaseForwarder(clinMgnager, "admin-service", pbAdmin.NewAdminServiceClient),
	}
}

// log
/*func (a *AdminForwarder) Login(ctx context.Context, req *pbAdmin.LoginRequest) (*pbAdmin.LoginResponse, error) {
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
}*/

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

func (a *AdminForwarder) GetUserInfo(ctx context.Context, req *pbUser.GetUserInfoRequest) (*pbUser.GetUserInfoResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetUserInfo(ctx, req)
}

func (a *AdminForwarder) GetUserList(ctx context.Context, req *pbUser.GetUserListRequest) (*pbUser.GetUserListResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.GetUserList(ctx, req)
}

func (a *AdminForwarder) Chat(ctx context.Context, req *pbLlm.ChatRequest) (*pbLlm.ChatResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.Chat(ctx, req)
}

// OpenStreamChat opens the AdminService stream used by the HTTP-to-SSE adapter.
func (a *AdminForwarder) OpenStreamChat(ctx context.Context, req *pbLlm.StreamChatRequest) (pbAdmin.AdminService_StreamChatClient, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is nil"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	client, err := a.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.StreamChat(ctx, req)
}

func (a *AdminForwarder) StreamChat(req *pbLlm.StreamChatRequest, stream pbAdmin.AdminService_StreamChatServer) error {
	if req == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	if stream == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream is nil"))
	}
	clientStream, err := a.OpenStreamChat(stream.Context(), req)
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

// RefreshToken
/*func (a *AdminForwarder) RefreshToken(ctx context.Context, req *pbAdmin.RefreshTokenRequest) (*pbAdmin.RefreshTokenResponse, error) {
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
*/
