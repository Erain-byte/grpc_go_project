package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

func (s *HTTPServer) registerAdminRoutes() {
	adminForwarder := forwarder.NewAdminForwarder(s.svcCtx, s.clientManager)
	admin := s.engine.Group("/admin")

	// 登录和刷新需要在没有 Access Token 时访问，因此不挂 JWT 中间件。
	admin.POST(
		"/login",
		handler.NewGrpcHandler[
			pbAdmin.LoginRequest,
			pbAdmin.LoginResponse,
		](adminForwarder.Login).Handle,
	)
	admin.POST(
		"/refresh",
		handler.NewGrpcHandler[
			pbAdmin.RefreshTokenRequest,
			pbAdmin.RefreshTokenResponse,
		](adminForwarder.RefreshToken).Handle,
	)

	// 组内路由统一先执行 JWTMiddleware.Handle，验证失败时不会进入 Handler。
	protected := admin.Group("")
	protected.Use(s.jwtMiddleware.Handle)
	protected.POST(
		"/logout",
		handler.NewGrpcHandler[
			pbAdmin.LogoutRequest,
			pbAdmin.LogoutResponse,
		](adminForwarder.Logout).Handle,
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
}
