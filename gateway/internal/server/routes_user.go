package server

import (
	"gateway/internal/forwarder"
	"gateway/internal/handler"
	"gateway/internal/middleware"

	pbLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	pbUser "github.com/Erain-byte/grpc_go_project/proto/user/v1"
	pbWeb3 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
)

func (s *HTTPServer) registerUserRoutes() {
	userForwarder := forwarder.NewUserForwarder(s.clientManager)
	streamHandler := handler.NewLlmHTTPHandler(userForwarder.OpenStreamChat)
	user := s.engine.Group("/user")

	// TRON USDT 授权查询是产品的匿名公开入口，不要求 Keycloak access token。
	// 仍然启用基于客户端 IP 的限流，避免公开接口被批量滥用。
	s.engine.POST(
		"/v1/tron/usdt/approvals/check",
		s.rateLimitMiddleware.Handle,
		handler.NewGrpcHandler[
			pbWeb3.CheckTronUSDTApprovalsRequest,
			pbWeb3.CheckTronUSDTApprovalsResponse,
		](userForwarder.CheckTronUSDTApprovals).Handle,
	)

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

	web3 := protected.Group("/web3")
	web3.POST(
		"/chains",
		handler.NewGrpcHandler[pbWeb3.GetSupportedChainsRequest, pbWeb3.GetSupportedChainsResponse](userForwarder.GetSupportedChains).Handle,
	)
	web3.POST(
		"/wallet/nonce",
		handler.NewGrpcHandler[pbWeb3.CreateBindingNonceRequest, pbWeb3.CreateBindingNonceResponse](userForwarder.CreateBindingNonce).Handle,
	)
	web3.POST(
		"/wallet/bind",
		handler.NewGrpcHandler[pbWeb3.BindWalletRequest, pbWeb3.BindWalletResponse](userForwarder.BindWallet).Handle,
	)
	web3.POST(
		"/wallet/unbind",
		handler.NewGrpcHandler[pbWeb3.UnbindWalletRequest, pbWeb3.UnbindWalletResponse](userForwarder.UnbindWallet).Handle,
	)
	web3.POST(
		"/wallet/list",
		handler.NewGrpcHandler[pbWeb3.GetWalletsRequest, pbWeb3.GetWalletsResponse](userForwarder.GetWallets).Handle,
	)
	web3.POST(
		"/wallet/update",
		handler.NewGrpcHandler[pbWeb3.UpdateWalletRequest, pbWeb3.UpdateWalletResponse](userForwarder.UpdateWallet).Handle,
	)
	web3.POST(
		"/balance",
		handler.NewGrpcHandler[pbWeb3.GetBalanceRequest, pbWeb3.GetBalanceResponse](userForwarder.GetBalance).Handle,
	)
	web3.POST(
		"/transaction",
		handler.NewGrpcHandler[pbWeb3.GetTransactionRequest, pbWeb3.GetTransactionResponse](userForwarder.GetTransaction).Handle,
	)
	web3.POST(
		"/transactions",
		handler.NewGrpcHandler[pbWeb3.GetTransactionsRequest, pbWeb3.GetTransactionsResponse](userForwarder.GetTransactions).Handle,
	)

}
