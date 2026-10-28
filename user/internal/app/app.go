package app

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user/internal/config"
	"user/internal/consul"
	"user/internal/databases"
	grpcclient "user/internal/grpc"
	"user/internal/idgen"
	"user/internal/logger"
	"user/internal/redis"
	"user/internal/server"
	"user/internal/sharding"
	"user/internal/svc"
	"user/internal/tracer"
	"user/pkg/apperror"
)

const dependencyCheckTimeout = 3 * time.Second

// Run 是 Admin 服务的启动入口。
func Run() error {
	//指定config文件路径
	var configFile string
	//创建命令行参数
	flag.StringVar(&configFile, "f", "etc/user.yaml", "path to config file")
	flag.Parse()
	//加载配置文件
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
	redisClient := redis.NewRedisClient(cfg.Redis)
	//关闭redis连接
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.SugaredLogger.Errorf("Failed to close redis client: %v", err)
		}
	}()
	//确定rdis连接正常
	if err := redisPing(context.Background(), redisClient); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to ping redis",
			http.StatusInternalServerError,
		)
	}
	//启动mysql
	GormClient, err := databases.NewGormClient(cfg.Database)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize mysql",
			http.StatusInternalServerError,
		)
	}
	//关闭mysql连接
	defer func() {
		if err := GormClient.Close(); err != nil {
			logger.SugaredLogger.Errorf("Failed to close mysql client: %v", err)
		}
	}()
	//测试数据库链接
	if err := databasePing(context.Background(), GormClient); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to ping mysql",
			http.StatusInternalServerError,
		)
	}
	//启动counsl
	consulClenit, err := consul.NewConsulRegistry(cfg.Consul)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize consul",
			http.StatusInternalServerError,
		)
	}
	//关闭consul连接
	defer consulClenit.Close()
	//测试consul连接
	if err := consulPing(context.Background(), consulClenit); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to ping consul",
			http.StatusInternalServerError,
		)
	}
	//创建Router
	router, err := sharding.NewUserRouter(cfg.Database.Sharding)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize router",
			http.StatusInternalServerError,
		)
	}
	//创建ids
	ids, err := idgen.NewSnowflake(cfg.IDGenerator.NodeID)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize id generator",
			http.StatusInternalServerError,
		)
	}
	//创建 gRPC 客户端管理器
	grpcClientManager := grpcclient.NewClientManager(
		consulClenit,
		&grpcclient.GrpcConfig{
			UseTLS:     cfg.GRPC.UseTLS,
			CertFile:   cfg.GRPC.CertFile,
			KeyFile:    cfg.GRPC.KeyFile,
			CaFile:     cfg.GRPC.ClientCAFile,
			ServerName: cfg.GRPC.TLSServerName,
		},
	)
	defer grpcClientManager.Close()
	//启动svc
	svc := svc.NewServiceContext(
		cfg,
		consulClenit,
		redisClient,
		GormClient,
		router,
		ids,
		logger.Logger,
		grpcClientManager,
	)
	//初始化trace
	traceCtx, traceCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer traceCancel()
	traceClient, traceErr := tracer.NewTracerProvider(traceCtx, cfg.Tracing)
	if traceErr != nil {
		return apperror.Wrap(
			traceErr,
			apperror.CodeInternal,
			"failed to initialize tracer",
			http.StatusInternalServerError,
		)
	}
	//关闭trace
	defer func() {
		tracingCtx, tracingCtxCancel := context.WithTimeout(
			context.Background(),
			dependencyCheckTimeout,
		)
		defer tracingCtxCancel()
		if err := traceClient.Shutdown(tracingCtx); err != nil {
			logger.SugaredLogger.Errorf("Failed to gracefully shutdown tracer: %v", err)
		}
	}()
	//初始化grpc
	grpcclient, err := server.NewGrpcServer(cfg, svc)
	if err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to initialize grpc server",
			http.StatusInternalServerError,
		)
	}
	//统一的异常退出
	var severErr = make(chan error, 1)
	//协程启动grpc
	go func() {
		//将启动状态写入severErr
		severErr <- grpcclient.Start()
	}()
	//grpc注册consul
	if err := consulClenit.RegisterService(
		cfg.Name,
		cfg.Host,
		cfg.GRPCPort,
		cfg.GRPC.UseTLS,
		cfg.GRPC.TLSServerName,
	); err != nil {
		return apperror.Wrap(
			err,
			apperror.CodeInternal,
			"failed to register service to consul",
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
	// 注销前停止维护，避免刚注销又被后台注册回来。
	stopRegistration := consulClenit.StartRegistrationMaintenance(signlCatxh, logger.Logger)
	//兜底异常退出依然动态补注册
	defer stopRegistration()
	//监听证书
	go grpcclient.MonitorCertificate(
		signlCatxh,
		12*time.Hour,
		logger.Logger,
	)
	//监听退出信号
	var runErr error
	select {
	case <-signlCatxh.Done():
		//接收到退出信号写入日志
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
	//手动关闭动态注册
	stopRegistration()
	//移除consul注册
	deregisterErr := consulClenit.DeregisterGRPC(cfg.Name, cfg.Host, cfg.GRPCPort)
	if deregisterErr != nil {
		logger.SugaredLogger.Errorf("Failed to deregister service from consul: %v", deregisterErr)
		runErr = errors.Join(runErr, deregisterErr)
	}
	closeCtx, closeCancel := context.WithTimeout(context.Background(), dependencyCheckTimeout)
	defer closeCancel()
	//关闭grpc
	if err := grpcclient.Stop(closeCtx); err != nil {
		logger.SugaredLogger.Errorf("Failed to gracefully shutdown grpc server: %v", err)
		runErr = errors.Join(runErr, err)
	}
	return runErr
}

// 辅助函数
// consulPing
func consulPing(ctx context.Context, consulClient *consul.ConsulRegistry) error {
	if consulClient == nil {
		return errors.New("consul is empty")
	}
	consulCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()
	return consulClient.Ping(consulCtx)
}
func redisPing(ctx context.Context, redisClient redis.RedisClient) error {

	if redisClient == nil {
		return errors.New("redis is empty")
	}
	redisCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()
	return redisClient.Ping(redisCtx)
}

// databsePing
func databasePing(ctx context.Context, GormClient *databases.GormClient) error {
	if GormClient == nil {
		return errors.New("mysql is empty")
	}
	gormCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()
	return GormClient.Ping(gormCtx)
}
