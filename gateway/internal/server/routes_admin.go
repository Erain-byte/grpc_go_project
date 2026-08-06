package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
)

func (s *HTTPServer) registerAdminRoutes() {
	adminForwarder := forwarder.NewAdminForwarder(s.svcCtx, s.clientManager)
	admin := s.engine.Group("/admin")
	//公开的接口
	public := admin.Group("")
	{
		public.POST(
			"/login",
			handler.NewGrpcHandler[
				pbAdmin.LoginRequest,
				pbAdmin.LoginResponse,
			](adminForwarder.Login).Handle,
		)
		//后续添加
	}

	//私有的接口
	protected := admin.Group("")
	protected.Use() //验证中间件还未开发
	{
		protected.POST("/logout", handler.NewGrpcHandler[
			pbAdmin.LogoutRequest,
			pbAdmin.LogoutResponse,
		](adminForwarder.Logout).Handle)
		protected.POST("/info", handler.NewGrpcHandler[
			pbAdmin.GetAdminInfoRequest,
			pbAdmin.GetAdminInfoResponse,
		](adminForwarder.GetAdminInfo).Handle)
		protected.POST("/create", handler.NewGrpcHandler[
			pbAdmin.CreateAdminRequest,
			pbAdmin.CreateAdminResponse,
		](adminForwarder.CreateAdmin).Handle)
		protected.POST(
			"/list",
			handler.NewGrpcHandler[
				pbAdmin.GetAdminListRequest,
				pbAdmin.GetAdminListResponse,
			](adminForwarder.GetAdminList).Handle)
	}

}
