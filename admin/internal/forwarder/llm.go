package forwarder

//LLmllmForwarder管理器
import (
	"admin/internal/grpcclient"
	"context"
	"io"

	apperror "admin/pkg/apperorr"

	bpLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
)

// NewLlmForwarder 创建llm转发器
type LlmForwarder struct {
	bpLlm.UnimplementedLlmServiceServer
	base *BaseForwarder[bpLlm.LlmServiceClient]
}

// 构造函数
func NewLlmForwarder(clientMager *grpcclient.ClientManager) *LlmForwarder {
	return &LlmForwarder{
		base: NewBaseForwarder(clientMager, "llm-service", bpLlm.NewLlmServiceClient),
	}
}

// Chat 实现llm服务的接口
func (f *LlmForwarder) Chat(ctx context.Context, req *bpLlm.ChatRequest) (*bpLlm.ChatResponse, error) {

	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}

	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("req is empty"))
	}
	client, err := f.base.Forward(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return client.Chat(ctx, req)
}

// StreamChat
func (f *LlmForwarder) StreamChat(req *bpLlm.StreamChatRequest, stream bpLlm.LlmService_StreamChatServer) error {
	if req == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("req is empty"))
	}
	if stream == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream is empty"))
	}
	ctx := stream.Context()
	client, err := f.base.Forward(ctx)
	if err != nil {
		return apperror.ToGRPC(err)

	}
	respStream, err := client.StreamChat(ctx, req) //调用服务端接口

	if err != nil {
		return apperror.ToGRPC(err)
	}

	for {
		resp, err := respStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return apperror.ToGRPC(err)
		}
		if err := stream.Send(resp); err != nil {
			return apperror.ToGRPC(err)
		}
		if resp.GetFinished() {
			return nil
		}
	}
}
