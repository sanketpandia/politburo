package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/routes"
)

// Vizburo UI service — serves the HTMX+Tailwind dashboard on a separate port.
// Shares PostgreSQL, Redis, and all DI-wired dependencies with the main API server.
func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg := app.LoadConfig()

	if err := logging.Init(cfg.AppEnv); err != nil {
		log.Fatalf("failed to initialize logging: %v", err)
	}
	defer logging.Close()

	application, err := app.New(cfg)
	if err != nil {
		logging.Error("Failed to initialize application", "error", err)
		log.Fatalf("failed to initialize application: %v", err)
	}

	router := routes.NewRouter(application)

	vizburoPort := os.Getenv("VIZBURO_PORT")
	if vizburoPort == "" {
		vizburoPort = "3000"
	}

	srv := &http.Server{
		Addr:         ":" + vizburoPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logging.Info("Vizburo UI starting", "address", srv.Addr)
		log.Printf("Vizburo UI listening on %s", srv.Addr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		logging.Error("Vizburo server error", "error", err)
		log.Fatalf("server error: %v", err)
	case sig := <-shutdown:
		logging.Info("Shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logging.Error("Vizburo HTTP server shutdown error", "error", err)
	}
	application.Shutdown(shutdownCtx)
	logging.Info("Vizburo shutdown complete")
}
