package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"
	"gateway/internal/middleware"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

func (s *HTTPServer) registerUserRoutes() {
	userForwarder := forwarder.NewUserForwarder(s.clientManager)
	streamHandler := handler.NewLlmHTTPHandler(userForwarder.OpenStreamChat)
	user := s.engine.Group("/user")

	// Every User API requires a valid Keycloak access token.
	protected := user.Group("")
	protected.Use(
		s.jwtMiddleware.Handle,
		middleware.RequireAnyRole(s.svcCtx.Config.Auth.AccessToken.RolePriority...),
		s.rateLimitMiddleware.Handle,
	)
	protected.POST(
		"/info",
		handler.NewGrpcHandler[
			pbUser.GetUserInfoRequest,
			pbUser.GetUserInfoResponse,
		](userForwarder.GetUserInfo).Handle,
	)
	protected.POST(
		"/chat",
		handler.NewGrpcHandler[
			pbLlm.ChatRequest,
			pbLlm.ChatResponse,
		](userForwarder.Chat).Handle,
	)
	protected.POST(
		"/initialize",
		handler.NewGrpcHandler[
			pbUser.InitializeUserRequest,
			pbUser.InitializeUserResponse,
		](userForwarder.InitializeUser).Handle,
	)
	protected.POST("/stream-chat", streamHandler.StreamChat)

}
