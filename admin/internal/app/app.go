// Package app assembles Admin service dependencies and owns their lifecycle.
package app

import (
	"admin/internal/config"
	"admin/internal/logger"
	"flag"
	"log"
)

// Run starts the Admin service. Dependencies will be added incrementally as
// configuration, persistence, and the gRPC server are implemented.
func Run() error {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/admin.yaml", "path to config file") //&configFile, "config", "", "path to config file"
	flag.Parse()
	config, err := config.InitConfig(configFile) //配置文件生效
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	//初始化日志
	if err := logger.InitializeLogger(config.Logger); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	//初始化数据库
	return nil
}
