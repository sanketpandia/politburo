package main

import (
	"context"
	"log"
	"os"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/runtime"
)

// @title Infinite Experiment API
// @version 1.0
// @description Backend for Infinite Experiment bot and web client.
// @contact.name Sanket Pandia
// @contact.email sanket@example.com
// @host localhost:8080
// @BasePath /
func main() {
	application, _, cleanup, err := runtime.Bootstrap()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer cleanup()

	if err := runtime.NewAPIServer(application).Run(context.Background()); err != nil {
		logging.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
