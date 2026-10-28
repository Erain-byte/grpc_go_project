package main

import (
	"log/slog"
	"os"
	"user/internal/app"
)

// main 暂时保持为空；服务依赖组装完成后由这里调用 app.Run。
func main() {
	if err := app.Run(); err != nil {
		slog.Error("user service stopped", "error", err)
		os.Exit(1)
	}
}
