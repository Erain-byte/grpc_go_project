// Command admin starts the Admin gRPC service.
package main

import (
	"log/slog"
	"os"

	"admin/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("admin service stopped", "error", err)
		os.Exit(1)
	}
}
