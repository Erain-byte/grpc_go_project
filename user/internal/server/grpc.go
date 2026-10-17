package server

import (
	"net"
	"strconv"

	"user/internal/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// 服务端
type GrpcServer struct {
	server *grpc.Server
	//健康检查
	health *health.Server
	//端口监听
	listenet net.Listener
	//证书
	cerFile string
}

// 构造函数
func NewGrpcServer(cfg *config.Config) (*GrpcServer, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.GRPCPort))
	//监听端口
	listenet, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	//grpc服务先定义opt
	options := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(options...)
	//健康检查
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	//注册服务handler还未完成
	//userpb.RegisterUserServiceServer(grpcServer)
	return &GrpcServer{
		server:   grpcServer,
		health:   healthServer,
		listenet: listenet,
	}, nil

}
