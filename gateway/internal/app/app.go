// Package app 负责组装网关依赖并管理应用生命周期。
package app

import "gateway/internal/config"

// Run 启动网关应用。
func Run() error {
	_ = config.Load()
	return nil
}
