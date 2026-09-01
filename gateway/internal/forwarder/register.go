package forwarder

import (
	"gateway/internal/logger"

	clien "gateway/internal/grpc"

	pbAdmin "github.com/Erain-byte/grpc_go_project/proto/admin/v1"
	bpLlm "github.com/Erain-byte/grpc_go_project/proto/llm/v1"
	"google.golang.org/grpc"
)

/**
 * @Description:注册GRPC服务
**/

// RegisterAllGRPCServices 注册所有 gRPC 服务到网关
func RegisterAllGRPCServices(grpcSrvc *grpc.Server, grpcClien *clien.ClientManager) {
	// 创建各个服务的转发器
	AdminForwarder := NewAdminForwarder(grpcClien)
	LlmForwarder := NewLlmForwarder(grpcClien)
	// 注册服务到网关
	pbAdmin.RegisterAdminServiceServer(grpcSrvc, AdminForwarder)
	logger.SugaredLogger.Infof("Admin service registered")
	bpLlm.RegisterLlmServiceServer(grpcSrvc, LlmForwarder)
	logger.SugaredLogger.Infof("Llm service registered")
	//后续注册的服务。。。
}
