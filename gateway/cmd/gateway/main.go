package main

import (
	"log"
	"os"

	"gateway/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Printf("gateway stopped: %v", err)
		os.Exit(1)
	}
}
