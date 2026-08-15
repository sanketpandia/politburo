package main

import (
	"context"
	"log/slog"
	"os"

	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/config"
	apphttp "infinite-experiment/politburo/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	server := apphttp.NewServer(application)
	if err := server.Run(context.Background()); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}
