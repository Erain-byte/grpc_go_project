// Package config 提供应用配置的加载与管理能力。
package config

import "os"

// Config 描述网关的运行时配置。
type Config struct {
	Environment string
}

// Load 从环境变量读取配置，并为缺失项设置默认值。
func Load() Config {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		environment = "development"
	}

	return Config{Environment: environment}
}
