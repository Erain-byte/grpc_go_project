package forwarder

import (
	"context"
	clien "gateway/internal/grpc"
	"gateway/pkg/apperror"
	"io"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
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
