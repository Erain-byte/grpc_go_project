// Package app 负责组装 Admin 服务依赖，并统一管理所有资源的生命周期。
package app

import (
	"admin/internal/config"
	"admin/internal/consul"
	"admin/internal/database"
	"admin/internal/logger"
	"admin/internal/mq"
	"admin/internal/mqhandler"
	"admin/internal/redis"
	"admin/internal/repository"
	"admin/internal/server"
	"admin/internal/svc"
	"admin/internal/tracer"
	apperror "admin/pkg/apperorr"
	"context"
	"errors"
	"flag"
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
	// RabbitMQ 是可选依赖。启用后共用一条 Connection，Publisher 和 Consumer
	// 分别持有自己的 Channel，避免在并发发布和消费之间共享 Channel 状态。
	var mqClient mq.Client
	var operationLogPublisher *mq.Publisher
	if cfg.RabbitMQ.Enabled {
		var mqErr error
		mqClient, mqErr = mq.NewClient(cfg.RabbitMQ)
		if mqErr != nil {
			return apperror.Wrap(mqErr, apperror.CodeUnavailable, "failed to initialize RabbitMQ", http.StatusServiceUnavailable)
		}
		// defer 按后进先出执行：Publisher Channel 会先关闭，底层 Connection 后关闭。
		defer func() {
			if err := mqClient.Close(); err != nil {
				logger.SugaredLogger.Errorf("Failed to close RabbitMQ client: %v", err)
			}
		}()
		if err := rabbitMQPing(context.Background(), mqClient); err != nil {
			return apperror.Wrap(err, apperror.CodeUnavailable, "failed to ping RabbitMQ", http.StatusServiceUnavailable)
		}
		operationLogPublisher, mqErr = mq.NewPublisher(mqClient, cfg.RabbitMQ)
		if mqErr != nil {
			return apperror.Wrap(mqErr, apperror.CodeUnavailable, "failed to initialize RabbitMQ publisher", http.StatusServiceUnavailable)
		}
		defer func() {
			if err := operationLogPublisher.Close(); err != nil {
				logger.SugaredLogger.Errorf("Failed to close RabbitMQ publisher: %v", err)
			}
		}()
	}
	//svc
	severice := svc.NewServiceContext(
		cfg,
		redisClinet,
		gormDB,
		logger.Logger,
		consulClenit,
		operationLogPublisher,
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
	if cfg.RabbitMQ.Enabled {
		// 消费链路：Consumer 解码消息 -> Handler 转换模型 -> Repository 幂等写库。
		operationLogRepository := repository.NewOperationLogRepository(severice)
		operationLogHandler, err := mqhandler.NewOperationLog(operationLogRepository)
		if err != nil {
			return apperror.Wrap(err, apperror.CodeInternal, "failed to initialize operation-log handler", http.StatusInternalServerError)
		}
		operationLogConsumer, err := mq.NewConsumer(
			mqClient,
			cfg.RabbitMQ,
			operationLogHandler,
			operationLogPublisher,
			logger.Logger,
		)
		if err != nil {
			return apperror.Wrap(err, apperror.CodeInternal, "failed to initialize RabbitMQ consumer", http.StatusInternalServerError)
		}
		go func() {
			// Consumer 内部会持续重连；这里只需要让它跟随服务退出 Context 结束。
			if err := operationLogConsumer.Start(signlCatxh); err != nil {
				logger.SugaredLogger.Errorf("RabbitMQ operation-log consumer stopped: %v", err)
			}
		}()
	}
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

func rabbitMQPing(ctx context.Context, client mq.Client) error {
	// 启动阶段执行一次快速健康检查，Broker 不可用时直接阻止服务进入可用状态。
	if client == nil {
		return errors.New("RabbitMQ client is nil")
	}
	pingCtx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()
	return client.Ping(pingCtx)
}
