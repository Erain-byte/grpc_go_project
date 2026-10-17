package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"
	"gateway/internal/middleware"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

func (s *HTTPServer) registerAdminRoutes() {
	adminForwarder := forwarder.NewAdminForwarder(s.clientManager)
	streamHandler := handler.NewLlmHTTPHandler(adminForwarder.OpenStreamChat)
	admin := s.engine.Group("/admin")

	// 组内路由统一先执行 JWTMiddleware.Handle，验证失败时不会进入 Handler。
	protected := admin.Group("")
	adminRoles := s.svcCtx.Config.Auth.AccessToken.RolesMatching("admin")
	protected.Use(
		s.jwtMiddleware.Handle,
		middleware.RequireAnyRole(adminRoles...),
		s.rateLimitMiddleware.Handle,
	)
	protected.POST(
		"/info",
		handler.NewGrpcHandler[
			pbAdmin.GetAdminInfoRequest,
			pbAdmin.GetAdminInfoResponse,
		](adminForwarder.GetAdminInfo).Handle,
	)
	protected.POST(
		"/create",
		handler.NewGrpcHandler[
			pbAdmin.CreateAdminRequest,
			pbAdmin.CreateAdminResponse,
		](adminForwarder.CreateAdmin).Handle,
	)
	protected.POST(
		"/list",
		handler.NewGrpcHandler[
			pbAdmin.GetAdminListRequest,
			pbAdmin.GetAdminListResponse,
		](adminForwarder.GetAdminList).Handle,
	)
	protected.POST(
		"/users/info",
		handler.NewGrpcHandler[
			pbUser.GetUserInfoRequest,
			pbUser.GetUserInfoResponse,
		](adminForwarder.GetUserInfo).Handle,
	)
	protected.POST(
		"/users/list",
		handler.NewGrpcHandler[
			pbUser.GetUserListRequest,
			pbUser.GetUserListResponse,
		](adminForwarder.GetUserList).Handle,
	)
	protected.POST(
		"/chat",
		handler.NewGrpcHandler[
			pbLlm.ChatRequest,
			pbLlm.ChatResponse,
		](adminForwarder.Chat).Handle,
	)
	protected.POST("/stream-chat", streamHandler.StreamChat)
}
