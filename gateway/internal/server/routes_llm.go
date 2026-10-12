package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"
	"gateway/internal/middleware"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
)

func (s *HTTPServer) registerLLMRoutes() {
	llmForwarder := forwarder.NewLlmForwarder(s.clientManager)
	llmHTTPHandler := handler.NewLlmHTTPHandler(llmForwarder.OpenStreamChat)
	llm := s.engine.Group("/llm")
	llm.Use(
		s.jwtMiddleware.Handle,
		middleware.RequireAnyRole(s.svcCtx.Config.Auth.AccessToken.RolePriority...),
		s.rateLimitMiddleware.Handle,
	)
	llm.POST("/chat", handler.NewGrpcHandler[
		pbLlm.ChatRequest,
		pbLlm.ChatResponse,
	](llmForwarder.Chat).Handle)
	llm.POST("/stream-chat", llmHTTPHandler.StreamChat)
	llm.POST("/chat-history", handler.NewGrpcHandler[
		pbLlm.GetChatHistoryRequest,
		pbLlm.GetChatHistoryResponse,
	](llmForwarder.GetChatHistory).Handle)
	llm.POST("/chat-list", handler.NewGrpcHandler[
		pbLlm.GetChatListRequest,
		pbLlm.GetChatListResponse,
	](llmForwarder.GetChatList).Handle)
	llm.POST("/call-model", handler.NewGrpcHandler[
		pbLlm.CallModelRequest,
		pbLlm.CallModelResponse,
	](llmForwarder.CallModel).Handle)
}
