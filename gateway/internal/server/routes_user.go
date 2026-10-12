package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"
	"gateway/internal/middleware"

	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
)

func (s *HTTPServer) registerUserRoutes() {
	userForwarder := forwarder.NewUserForwarder(s.clientManager)
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

	// User administration requires an administrator role.
	adminRoles := s.svcCtx.Config.Auth.AccessToken.RolesMatching("admin")
	administration := user.Group("")
	administration.Use(
		s.jwtMiddleware.Handle,
		middleware.RequireAnyRole(adminRoles...),
		s.rateLimitMiddleware.Handle,
	)
	administration.POST(
		"/create",
		handler.NewGrpcHandler[
			pbUser.CreateUserRequest,
			pbUser.CreateUserResponse,
		](userForwarder.CreateUser).Handle,
	)
	administration.POST(
		"/list",
		handler.NewGrpcHandler[
			pbUser.GetUserListRequest,
			pbUser.GetUserListResponse,
		](userForwarder.GetUserList).Handle,
	)
}
