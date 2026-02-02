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

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// @title Infinite Experiment API
// @version 1.0
// @description Backend for Infinite Experiment bot and web client.
// @contact.name Sanket Pandia
// @contact.email sanket@example.com
// @host localhost:8080
// @BasePath /
func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// ==========================================
	// Phase 1: Load Configuration
	// ==========================================
	cfg := app.LoadConfig()
	log.Printf("✓ Configuration loaded (env=%s, port=%s)", cfg.AppEnv, cfg.Port)

	// ==========================================
	// Phase 2: Initialize Logging
	// ==========================================
	if err := logging.Init(cfg.AppEnv); err != nil {
		log.Fatalf("❌ Failed to initialize logging: %v", err)
	}
	defer logging.Close()

	logging.Info("Politburo starting up",
		"environment", cfg.AppEnv,
		"debug", cfg.Debug,
		"timestamp", time.Now().Format(time.RFC3339),
	)

	// ==========================================
	// Phase 3: Initialize Application (DI)
	// ==========================================
	application, err := app.New(cfg)
	if err != nil {
		logging.Error("Failed to initialize application", "error", err)
		log.Fatalf("❌ Failed to initialize application: %v", err)
	}
	logging.Info("Application initialized successfully")

	// ==========================================
	// Phase 4: Setup Routes
	// ==========================================
	router := routes.NewRouter(application)
	logging.Info("Router configured with all routes")

	// Setup metrics endpoint outside of Chi router
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", router) // Mount Chi router at root
	logging.Info("Prometheus metrics endpoint registered at /metrics")

	// ==========================================
	// Phase 5: Register and Start Scheduled Jobs
	// ==========================================
	if err := routes.RegisterScheduledJobs(application); err != nil {
		logging.Error("Failed to register scheduled jobs", "error", err)
		log.Fatalf("❌ Failed to register jobs: %v", err)
	}
	application.Infra.Scheduler.Start()
	logging.Info("Scheduler started with registered jobs")

	// ==========================================
	// Phase 5b: Register and Start Background Workers
	// ==========================================
	if err := routes.RegisterWorkers(application); err != nil {
		logging.Error("Failed to register workers", "error", err)
		log.Fatalf("❌ Failed to register workers: %v", err)
	}
	logging.Info("Background workers started")

	// ==========================================
	// Phase 6: Configure HTTP Server
	// ==========================================
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logging.Info("HTTP server configured",
		"port", cfg.Port,
		"read_timeout", "15s",
		"write_timeout", "15s",
		"idle_timeout", "60s",
	)

	// ==========================================
	// Phase 7: Start Server and Wait for Shutdown
	// ==========================================
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start HTTP server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logging.Info("HTTP server starting", "address", srv.Addr)
		log.Printf("🚀 Server listening on %s", srv.Addr)
		serverErr <- srv.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		logging.Error("Server error", "error", err)
		log.Fatalf("❌ Server error: %v", err)
	case sig := <-shutdown:
		logging.Info("Shutdown signal received", "signal", sig.String())
		log.Printf("⚠️  Shutdown signal received: %s", sig.String())
	}

	// ==========================================
	// Phase 8: Graceful Shutdown
	// ==========================================
	logging.Info("Initiating graceful shutdown...")

	// Shutdown HTTP server with 30-second deadline
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logging.Error("HTTP server shutdown error", "error", err)
	} else {
		logging.Info("HTTP server stopped gracefully")
	}

	// Shutdown application resources (scheduler, redis, db)
	application.Shutdown(shutdownCtx)

	logging.Info("Shutdown complete")
	log.Println("✓ Politburo shutdown complete")
}
