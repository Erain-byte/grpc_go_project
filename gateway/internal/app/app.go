// Package app 负责组装网关依赖并管理应用生命周期。
package app

import (
	"context"
	"flag"
	"gateway/internal/config"
	"gateway/internal/consul"
	"gateway/internal/logger"
	"gateway/internal/service"
	"gateway/internal/svc"
	"log"
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
		log.Fatalf("Failed to initialize config: %v", err)
	}
	//初始化日志
	if err := logger.InitializeLogger(cfg.Logger); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
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
	//启动http服务
	srv := service.NewServer(serviceContext)
	go srv.Start()
	return nil
}
