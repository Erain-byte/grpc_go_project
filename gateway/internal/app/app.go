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
	redisclient "gateway/internal/redis"
	"gateway/internal/server"
	"gateway/internal/svc"
	"gateway/internal/tracer"
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
	//初始化链路

	tracerCtx, cancelTracer := context.WithTimeout(context.Background(), 5*time.Second)
	tracerManager, err := tracer.NewTracerProvider(tracerCtx, cfg.Tracing)
	cancelTracer()
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelShutdown()
		if err := tracerManager.Shutdown(shutdownCtx); err != nil {
			logger.SugaredLogger.Errorf("failed to shutdown tracer: %v", err)
		}
	}()

	// Redis 由 app 创建并管理生命周期。
	redisClient := redisclient.NewRedisClient(cfg.Redis)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.SugaredLogger.Errorf("failed to close Redis client: %v", err)
		}
	}()

	healthCtx, cancelHealth := context.WithTimeout(context.Background(), 3*time.Second)
	healthErr := redisClient.Ping(healthCtx)
	cancelHealth()
	if healthErr != nil {
		if cfg.Redis.HealthRequired || cfg.IsProduction() {
			return apperror.Wrap(
				healthErr,
				apperror.CodeUnavailable,
				"Redis health check failed",
				http.StatusServiceUnavailable,
			)
		}
		logger.SugaredLogger.Warnf("Redis health check failed, continuing because environment is not production: %v", healthErr)
	}

	//初始化consul服务
	consulRegistry, err := consul.NewConsulRegistry(cfg.Consul)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeUnavailable,
			"failed to initialize Consul registry",
			http.StatusServiceUnavailable,
		)
	}
	defer consulRegistry.Close()

	serviceContext := svc.NewServiceContext(*cfg, redisClient, consulRegistry)
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
	httpServer, err := server.NewHTTPServer(
		serviceContext,
		grpcClientManager, // 注入 grpcClientManager，供服务调用使用
	)
	if err != nil {
		return err
	}
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	grpcServer, err := server.NewGRPCServer(
		grpcAddr,
		serviceContext,
		grpcClientManager,
	)
	if err != nil {
		return err
	}

	// 将 HTTP 实例和 /health 检查地址注册到 Consul。
	// 注册成功后不是 Gateway 主动循环检查，而是 Consul Agent 按
	// check_interval 定期请求 http://host:port/health。
	if err := consulRegistry.RegisterHTTP(cfg.Name, cfg.Host, cfg.Port, cfg); err != nil {
		return err
	}

	// 将 gRPC 实例注册到 Consul。Consul Agent 会定期调用该端口上的
	// grpc.health.v1.Health 服务，因此 grpcServer 必须注册标准健康服务。
	if err := consulRegistry.RegisterGRPC(cfg.Name, cfg.Host, cfg.GRPCPort, cfg); err != nil {
		// HTTP 已经注册成功而 gRPC 注册失败时，立即撤销 HTTP 注册，
		// 避免 Consul 中留下一个实际上没有完整启动的 Gateway 实例。
		if deregisterErr := consulRegistry.DeregisterHTTPService(cfg.Name, cfg.Host, cfg.Port); deregisterErr != nil {
			return errors.Join(err, deregisterErr)
		}
		return err
	}
	// Run 退出时注销两个实例。defer 按后进先出执行，因此该注销动作
	// 会先于 consulRegistry.Close，保证注销时 Consul Client 仍然可用。
	defer func() {
		if err := consulRegistry.DeregisterGRPCService(cfg.Name, cfg.Host, cfg.GRPCPort); err != nil {
			logger.SugaredLogger.Errorf("failed to deregister Gateway gRPC service: %v", err)
		}
		if err := consulRegistry.DeregisterHTTPService(cfg.Name, cfg.Host, cfg.Port); err != nil {
			logger.SugaredLogger.Errorf("failed to deregister Gateway HTTP service: %v", err)
		}
	}()

	// 主 goroutine 统一等待退出信号或任意 Server 异常退出。
	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()
	//启动TLs证书自动检测
	go grpcClientManager.MonitorCertificate(
		signalCtx,
		12*time.Hour,
		logger.Logger,
	)

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
