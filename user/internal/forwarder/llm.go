package forwarder

import (
	"context"
	"io"
	"user/internal/grpc"
	"user/pkg/apperror"

	bpLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
)

// 创建Llm服务转发器
type LlmForwarder struct {
	base *BaseForwarder[bpLlm.LlmServiceClient]
}

// 构造函数
func NewLlmForwarder(clientMager *grpc.ClientManager) *LlmForwarder {
	return &LlmForwarder{
		base: NewBaseForwarder("llm-service", clientMager, bpLlm.NewLlmServiceClient),
	}
}

// 获取Llm服务转发器Chat方法
func (f *LlmForwarder) Chat(ctx context.Context, req *bpLlm.ChatRequest) (*bpLlm.ChatResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("req is empty"))
	}
	conn, err := f.base.Client(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	//返回调用结果并且把ctx 传递给下游
	return conn.Chat(OutgoingContext(ctx), req)
}

// 获取Llm服务转发器ChatStream方法
func (f *LlmForwarder) StreamChat(req *bpLlm.StreamChatRequest, stream bpLlm.LlmService_StreamChatServer) error {
	if req == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("req is empty"))
	}
	if stream == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream is empty"))
	}
	ctx := stream.Context()
	conn, err := f.base.Client(ctx)
	if err != nil {
		return apperror.ToGRPC(err)
	}
	respStream, err := conn.StreamChat(OutgoingContext(ctx), req)
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
		//成功收到数据转发给Gateway
		if err := stream.Send(resp); err != nil {
			return apperror.ToGRPC(err)
		}
		//判断是否结束
		if resp.GetFinished() {
			return nil
		}
	}

}
