package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"web3/internal/app"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "f", "etc/web3.yaml", "path to Web3 config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, configPath); err != nil {
		slog.Error("Web3 service stopped", "error", err)
		os.Exit(1)
	}
}
