// Package app 负责组装网关依赖并管理应用生命周期。
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"gateway/internal/config"
	"gateway/internal/consul"
	grpcclient "gateway/internal/grpc" //grpc客户端
	"gateway/internal/logger"
	"gateway/internal/server"
	"gateway/internal/svc"
	"gateway/pkg/apperror"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run 启动网关应用。
func Run() error {
	//初始化
	var configFlie string //文件路径
	flag.StringVar(&configFlie, "f", "etc/gateway.yaml", "config file path")
	flag.Parse() //
	cfg, err := config.InitConfig(configFlie)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize config",
			http.StatusInternalServerError,
		)
	}
	//初始化日志
	if err := logger.InitializeLogger(cfg.Logger); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize logger",
			http.StatusInternalServerError,
		)
	}
	//确保日志被写入磁盘
	defer logger.Sync()

	//初始化redis服务
	serviceContext := svc.NewServiceContext(*cfg)
	healthCtx, healthCncel := context.WithTimeout(context.Background(), time.Second*3)
	defer healthCncel()
	if err := serviceContext.HealthCheck(healthCtx); err != nil { //redis检查
		if cfg.Redis.HealthRequired || cfg.IsProduction() {
			logger.SugaredLogger.Fatalf("Redis health check failed: %v", err)
		}
		logger.SugaredLogger.Warnf("Redis health check failed, continuing because environment is not production: %v", err)
	}
	defer serviceContext.Close()
	//初始化consul服务
	consulRegistry, err := consul.NewConsulRegistry(cfg.Consul)
	if err != nil {
		logger.SugaredLogger.Fatalf("Failed to initialize consul registry: %v", err)
	}
	serviceContext.Registry = consulRegistry // 注入 Registry，供服务发现和生命周期管理使用
	//依赖注入ClientManager
	grpcClientManager := grpcclient.NewClientManager(
		consulRegistry,
		&grpcclient.GrpcConfig{
			UseTLS:             cfg.Grpc.UseTLS,
			InsecureSkipVerify: cfg.Grpc.InsecureSkipVerify,
			CertFile:           cfg.Grpc.CertFile,
			KeyFile:            cfg.Grpc.KeyFile,
			CaFile:             cfg.Grpc.CaFile,
			ServerName:         cfg.Grpc.ServerName,
		},
	)
	defer grpcClientManager.Close()

	// 创建 HTTP 和 gRPC 两个入站服务，它们共享同一个 ClientManager。
	httpServer := server.NewHTTPServer(
		serviceContext,
		grpcClientManager, // 注入 grpcClientManager，供服务调用使用
	)

	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	grpcServer, err := server.NewGRPCServer(
		grpcAddr,
		serviceContext,
		grpcClientManager,
	)
	if err != nil {
		return err
	}

	// 主 goroutine 统一等待退出信号或任意 Server 异常退出。
	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	type serverResult struct {
		name string
		err  error
	}
	serverErrors := make(chan serverResult, 2)

	go func() {
		serverErrors <- serverResult{name: "HTTP", err: httpServer.Start()}
	}()
	go func() {
		serverErrors <- serverResult{name: "gRPC", err: grpcServer.Start()}
	}()

	var runErr error
	select {
	case <-signalCtx.Done():
		logger.SugaredLogger.Info("shutdown signal received")
	case result := <-serverErrors:
		if result.err != nil {
			runErr = apperror.Wrap(
				result.err,
				apperror.CodeUnavailable,
				result.name+" server stopped",
				http.StatusServiceUnavailable,
			)
		} else {
			runErr = apperror.Unavailable(result.name + " server stopped unexpectedly")
		}
	}

	// 两个 Server 共用一个关闭期限，并发关闭可避免关闭时间相加。
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		parseShutdownTimeout(cfg.Shutdown.Timeout),
	)
	defer cancelShutdown()

	shutdownErrors := make(chan error, 2)
	go func() {
		shutdownErrors <- httpServer.Shutdown(shutdownCtx)
	}()
	go func() {
		shutdownErrors <- grpcServer.Shutdown(shutdownCtx)
	}()

	for range 2 {
		if err := <-shutdownErrors; err != nil {
			runErr = errors.Join(runErr, err)
		}
	}

	return runErr
}

func parseShutdownTimeout(value string) time.Duration {
	if value != "" {
		if timeout, err := time.ParseDuration(value); err == nil && timeout > 0 {
			return timeout
		}
	}
	return 10 * time.Second
}
