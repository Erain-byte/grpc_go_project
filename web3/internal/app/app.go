// Package app 组装 Web3 服务依赖并管理完整生命周期。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"web3/internal/config"
	"web3/internal/consul"
	"web3/internal/databases"
	"web3/internal/logger"
	redisstore "web3/internal/redis"
	"web3/internal/server"
	"web3/internal/svc"
	"web3/internal/tracer"

	"go.uber.org/zap"
)

const dependencyCheckTimeout = 5 * time.Second

func Run(ctx context.Context, configPath string) (runErr error) {
	cfg, err := config.InitConfig(configPath)
	if err != nil {
		return err
	}
	if err := logger.InitializeLogger(cfg.Logger); err != nil {
		return fmt.Errorf("initialize Web3 logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()
	log := logger.Logger.Named("app")
	tracingCtx, cancelTracing := context.WithTimeout(ctx, dependencyCheckTimeout)
	tracerManager, err := tracer.NewTracerProvider(tracingCtx, cfg.Tracing)
	cancelTracing()
	if err != nil {
		return fmt.Errorf("initialize Web3 tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), dependencyCheckTimeout)
		defer cancelShutdown()
		if err := tracerManager.Shutdown(shutdownCtx); err != nil {
			log.Error("shutdown Web3 tracing", zap.Error(err))
		}
	}()
	if !cfg.Database.Enabled || !cfg.Redis.Enabled {
		return errors.New("Web3 database and Redis must be enabled")
	}

	database, err := databases.NewGormClient(cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize Web3 database: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, database.Close()) }()

	redisClient := redisstore.NewClient(cfg.Redis)
	defer func() { runErr = errors.Join(runErr, redisClient.Close()) }()

	checkCtx, cancelChecks := context.WithTimeout(ctx, dependencyCheckTimeout)
	err = errors.Join(database.Ping(checkCtx), redisClient.Ping(checkCtx))
	cancelChecks()
	if err != nil {
		return fmt.Errorf("check Web3 dependencies: %w", err)
	}

	var consulRegistry *consul.ConsulRegistry
	if cfg.Consul.Enabled {
		consulRegistry, err = consul.NewConsulRegistry(cfg.Consul)
		if err != nil {
			return err
		}
		defer consulRegistry.Close()
		checkCtx, cancelChecks = context.WithTimeout(ctx, dependencyCheckTimeout)
		err = consulRegistry.Ping(checkCtx)
		cancelChecks()
		if err != nil {
			return fmt.Errorf("check Consul dependency: %w", err)
		}
	}

	serviceContext, err := svc.NewServiceContext(cfg, consulRegistry, redisClient, database, logger.Logger)
	if err != nil {
		return err
	}
	defer serviceContext.Blockchain.Close()
	// Scanner 使用链上 RPC 客户端；必须在 RPC 客户端关闭前停止后台扫描。
	stopScanner := serviceContext.Scanner.Start(ctx, cfg.Blockchain.Chains, logger.Logger.Named("scanner"))
	defer stopScanner()
	grpcServer, err := server.NewGRPCServer(cfg, serviceContext)
	if err != nil {
		return err
	}
	pprofServer, err := server.NewPprofServer(cfg.Pprof)
	if err != nil {
		return fmt.Errorf("create Web3 pprof server: %w", err)
	}

	type serverResult struct {
		name string
		err  error
	}
	serverCount := 1
	if pprofServer != nil {
		serverCount++
	}
	serverErr := make(chan serverResult, serverCount)
	go func() {
		serverErr <- serverResult{name: "gRPC", err: grpcServer.Start()}
	}()
	if pprofServer != nil {
		go func() {
			serverErr <- serverResult{name: "pprof", err: pprofServer.Run()}
		}()
	}

	advertiseHost := strings.TrimSpace(cfg.AdvertiseHost)
	if advertiseHost == "" {
		advertiseHost = strings.TrimSpace(cfg.Host)
	}
	registered := false
	stopMaintenance := func() {}
	if consulRegistry != nil {
		if err := consulRegistry.RegisterGRPC(cfg.Name, advertiseHost, cfg.GRPCPort, cfg.GRPC.UseTLS, cfg.GRPC.TLSServerName); err != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), dependencyCheckTimeout)
			_ = shutdownServers(shutdownCtx, grpcServer.Stop, pprofServerShutdown(pprofServer))
			cancel()
			return err
		}
		registered = true
		stopMaintenance = consulRegistry.StartRegistrationMaintenance(ctx, logger.Logger.Named("consul"))
	}
	log.Info("Web3 service started", zap.String("grpc_address", grpcServer.Address()))
	if pprofServer != nil {
		log.Info("Web3 pprof server started", zap.String("pprof_address", pprofServer.Address()))
	}

	select {
	case <-ctx.Done():
		// 操作系统退出信号属于正常关闭，不把 context.Canceled 当成服务错误。
	case result := <-serverErr:
		runErr = result.err
		if runErr != nil {
			log.Error("Web3 server stopped unexpectedly", zap.String("server", result.name), zap.Error(runErr))
		}
	}
	if runErr != nil {
		log.Error("Web3 service is stopping after server failure", zap.Error(runErr))
	} else {
		log.Info("Web3 service shutdown requested")
	}

	stopMaintenance()
	if registered {
		runErr = errors.Join(runErr, consulRegistry.DeregisterGRPC(cfg.Name, advertiseHost, cfg.GRPCPort))
	}
	shutdownTimeout, err := time.ParseDuration(cfg.Shutdown.Timeout)
	if err != nil || shutdownTimeout <= 0 {
		shutdownTimeout = 15 * time.Second
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	runErr = errors.Join(runErr, shutdownServers(
		shutdownCtx,
		grpcServer.Stop,
		pprofServerShutdown(pprofServer),
	))
	cancelShutdown()
	return runErr
}

func pprofServerShutdown(pprofServer *server.PprofServer) func(context.Context) error {
	if pprofServer == nil {
		return nil
	}
	return pprofServer.Shutdown
}

func shutdownServers(ctx context.Context, shutdowns ...func(context.Context) error) error {
	count := 0
	for _, shutdown := range shutdowns {
		if shutdown != nil {
			count++
		}
	}
	if count == 0 {
		return nil
	}

	errs := make(chan error, count)
	for _, shutdown := range shutdowns {
		if shutdown == nil {
			continue
		}
		go func(stop func(context.Context) error) {
			errs <- stop(ctx)
		}(shutdown)
	}

	var result error
	for range count {
		result = errors.Join(result, <-errs)
	}
	return result
}
