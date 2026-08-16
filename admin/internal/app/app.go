// Package app 负责组装 Admin 服务依赖，并统一管理所有资源的生命周期。
package app

import (
	"admin/internal/config"
	"admin/internal/consul"
	"admin/internal/database"
	"admin/internal/logger"
	"admin/internal/redis"
	"admin/internal/server"
	"admin/internal/svc"
	"admin/internal/tracer"
	"context"
	"errors"
	"flag"
	"gateway/pkg/apperror"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// dependencyCheckTimeout 限制依赖初始化和健康检查的最长等待时间，
// 防止数据库、Redis 或链路追踪服务异常时，进程永久卡在启动阶段。
const dependencyCheckTimeout = 3 * time.Second

// Run 是 Admin 服务的启动入口。
func Run() error {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/admin.yaml", "path to config file")
	flag.Parse()
	cfg, cfgErr := config.InitConfig(configFile)
	if cfgErr != nil {
		return apperror.Wrap(
			cfgErr,
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
	//初始redis
	redisClinet := redis.NewRedisClient(cfg.Redis)
	defer func() {
		if err := redisClinet.Close(); err != nil {
			logger.SugaredLogger.Errorf("Failed to close redis client: %v", err)
		}
	}()
	if err := redisPin(context.Background(), redisClinet); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize redis",
			http.StatusInternalServerError,
		)
	}
	//MYSQL
	gormDB, gormErr := database.NewGormClient(cfg.Database)
	if gormErr != nil {
		return apperror.Wrap(
			gormErr,
			apperror.CodeInternal,
			"failed to initialize gorm",
			http.StatusInternalServerError,
		)
	}
	defer func() {
		if err := gormDB.Close(); err != nil {
			logger.SugaredLogger.Errorf("Failed to close gorm client: %v", err)
		}
	}()
	if err := mysqlPin(context.Background(), gormDB); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize gorm",
			http.StatusInternalServerError,
		)
	}
	//conusl
	consulClenit, consulErr := consul.NewConsulRegistry(cfg.Consul)
	if consulErr != nil {
		return apperror.Wrap(
			consulErr,
			apperror.CodeInternal,
			"failed to initialize consul",
			http.StatusInternalServerError,
		)
	}
	defer consulClenit.Close()
	if err := consulPin(context.Background(), consulClenit); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize consul",
			http.StatusInternalServerError,
		)
	}
	//svc
	severice := svc.NewServiceContext(
		cfg,
		redisClinet,
		gormDB,
		logger.Logger,
		consulClenit,
	)
	tracerCtx, traceCtxCancel := context.WithTimeout(context.Background(), dependencyCheckTimeout)
	//tacer
	tracerClient, tracerErr := tracer.NewTracerProvider(tracerCtx, cfg.Tracing)
	traceCtxCancel()
	if tracerErr != nil {
		return apperror.Wrap(
			tracerErr,
			apperror.CodeInternal,
			"failed to initialize tracer",
			http.StatusInternalServerError,
		)
	}
	defer func() {
		tracingCtx, tracingCtxCancel := context.WithTimeout(
			context.Background(),
			dependencyCheckTimeout,
		)
		defer tracingCtxCancel()

		if err := tracerClient.Shutdown(tracingCtx); err != nil {
			logger.SugaredLogger.Errorf("Failed to shutdown tracer: %v", err)
		}
	}()

	//grcSever
	grpcServer, grpcErr := server.NewGRPCServer(cfg, severice)

	if grpcErr != nil {
		return apperror.Wrap(
			grpcErr,
			apperror.CodeInternal,
			"failed to initialize grpc server",
			http.StatusInternalServerError,
		)
	}
	var severErr = make(chan error, 1)
	go func() {
		severErr <- grpcServer.Start()
	}()
	//grpc注册consul
	if err := consulClenit.RegisterGRPC(
		cfg.Name,
		cfg.Host,
		cfg.GRPCPort,
		cfg.GRPC.UseTLS,
	); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to register grpc server",
			http.StatusInternalServerError,
		)
	}
	//优雅关闭
	signlCatxh, signalCtxCancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer signalCtxCancel()
	//监控证书
	go grpcServer.MonitorCertificate(
		signlCatxh,
		12*time.Hour,
		logger.Logger,
	)
	var runErr error
	select {
	case <-signlCatxh.Done():
		logger.SugaredLogger.Infof("Received OS shutdown signal: %v", signlCatxh.Err())
	case runErr = <-severErr:
		logger.SugaredLogger.Errorf("Failed to run grpc server: %v", runErr)
		runErr = apperror.Wrap(
			runErr,
			apperror.CodeInternal,
			"failed to run gRPC server",
			http.StatusInternalServerError,
		)
	}
	//注销GRPC
	deregisterErr := consulClenit.DeregisterGRPC(cfg.Name, cfg.Host, cfg.GRPCPort)
	if deregisterErr != nil {
		logger.SugaredLogger.Errorf("Failed to deregister grpc server: %v", deregisterErr)
		runErr = errors.Join(runErr, deregisterErr)
	}
	var stopErr = make(chan error, 1)
	stopCtx, stopCtxCancel := context.WithTimeout(context.Background(), dependencyCheckTimeout)
	defer stopCtxCancel()
	go func() {
		stopErr <- grpcServer.Stop(stopCtx)
	}()
	if err := <-stopErr; err != nil {
		logger.SugaredLogger.Errorf("Failed to stop grpc server: %v", err)
		errors.Join(runErr, err)
	}
	return runErr
}

func mysqlPin(ctx context.Context, gormDB *database.GormClient) error {
	if gormDB == nil {
		return errors.New("gorm is not client")
	}
	gormCtxh, gormCtxCancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer gormCtxCancel()
	return gormDB.Ping(gormCtxh)
}
func redisPin(ctx context.Context, redisClient redis.RedisClient) error {
	if redisClient == nil {
		return errors.New("redis is empty")
	}
	redisCtx, redisCtxCancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer redisCtxCancel()
	return redisClient.Ping(redisCtx)
}
func consulPin(ctx context.Context, consulClient *consul.ConsulRegistry) error {
	if consulClient == nil {
		return errors.New("conusl is empty")
	}
	consulCtx, consulCtxCancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer consulCtxCancel()
	return consulClient.Ping(consulCtx)
}
