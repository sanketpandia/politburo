package main

import (
	"context"
	"log"
	"os"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/runtime"
)

// Vizburo UI service — serves the HTMX+Tailwind dashboard on a separate port.
// Shares PostgreSQL, Redis, and all DI-wired dependencies with the main API server.
func main() {
	application, _, cleanup, err := runtime.Bootstrap()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer cleanup()

	if err := runtime.NewVizburoServer(application).Run(context.Background()); err != nil {
		logging.Error("vizburo exited with error", "error", err)
		os.Exit(1)
	}
}
