package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
)

func (s *HTTPServer) registerLLMRoutes() {
	llmForwarder := forwarder.NewLlmForwarder(s.svcCtx, s.clientManager)
	llmHTTPHandler := handler.NewLlmHTTPHandler(s.clientManager)
	// LLM 路由全部需要登录，统一挂载 JWT 中间件。
	llm := s.engine.Group("/llm")
	// JWT 验证通过后继续检查服务端 Session，退出的 Token 会在此处被拦截。
	llm.Use(s.jwtMiddleware.Handle, s.sessionMiddleware.Handle)

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
