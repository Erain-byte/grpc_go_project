package main

import (
	"fmt"
	"os"

	"gateway/internal/app"
	"gateway/internal/logger"
)

func main() {
	//初始化配置文件

	if err := app.Run(); err != nil {
		if logger.SugaredLogger != nil {
			logger.SugaredLogger.Errorf("gateway stopped: %v", err)
			_ = logger.Sync()
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "gateway stopped: %v\n", err)
		}
		os.Exit(1)
	}
}
