package forwarder

import (
	"context"
	"io"

	clien "gateway/internal/grpc"
	"gateway/internal/svc"
	"gateway/pkg/apperror"

	bpLlm "github.com/Erain-byte/grpc_go_project/proto/llm"
)

// llmForwarder管理器
type LlmForwarder struct {
	bpLlm.UnimplementedLlmServiceServer //实现llm服务的接口
	base                                *BaseForwarder[bpLlm.LlmServiceClient]
}

func NewLlmForwarder(svcCtx *svc.ServiceContext, clinMgnager *clien.ClientManager) *LlmForwarder {
	return &LlmForwarder{
		base: NewBaseForwarder[bpLlm.LlmServiceClient](
			svcCtx,
			bpLlm.NewLlmServiceClient,
			clinMgnager,
			"llm-service",
		),
	}
}

// Chat 实现llm服务的接口
func (l *LlmForwarder) Chat(ctx context.Context, req *bpLlm.ChatRequest) (*bpLlm.ChatResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	clien, err := l.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.Chat(ctx, req)
}

// StreamChat
func (l *LlmForwarder) StreamChat(req *bpLlm.StreamChatRequest, stream bpLlm.LlmService_StreamChatServer) error {
	if req == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	if stream == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream is nil"))
	}
	ctx := stream.Context()
	if ctx == nil {
		return apperror.ToGRPC(apperror.InvalidArgument("stream context is nil"))
	}

	client, err := l.base.GetClient(ctx) //获取客户端
	if err != nil {
		return apperror.ToGRPC(err)
	}
	respStream, err := client.StreamChat(ctx, req) //调用服务
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
		if err := stream.Send(resp); err != nil { //发送响应
			return apperror.ToGRPC(err)
		}
		if resp.GetFinished() { //判断是否结束
			return nil
		}
	}
}

//GetChatHistory

func (l *LlmForwarder) GetChatHistory(ctx context.Context, req *bpLlm.GetChatHistoryRequest) (*bpLlm.GetChatHistoryResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	clien, err := l.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.GetChatHistory(ctx, req)
}

// GetChatList
func (l *LlmForwarder) GetChatList(ctx context.Context, req *bpLlm.GetChatListRequest) (*bpLlm.GetChatListResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	clien, err := l.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.GetChatList(ctx, req)
}

// CallModel
func (l *LlmForwarder) CallModel(ctx context.Context, req *bpLlm.CallModelRequest) (*bpLlm.CallModelResponse, error) {
	if ctx == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("ctx is empty"))
	}
	if req == nil {
		return nil, apperror.ToGRPC(apperror.InvalidArgument("request is nil"))
	}
	clien, err := l.base.GetClient(ctx)
	if err != nil {
		return nil, apperror.ToGRPC(err)
	}
	return clien.CallModel(ctx, req)
}
