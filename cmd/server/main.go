package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"infinite-experiment/politburo/infra/db"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/routes"
	// Swagger docs
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

	// Initialize structured logging
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	if err := logging.Init(appEnv); err != nil {
		log.Fatalf("❌ Failed to initialize logger: %v", err)
	}
	defer logging.Close()

	logging.Info("Politburo starting up",
		"environment", appEnv,
		"timestamp", time.Now().Format(time.RFC3339),
	)

	// Metrics registry will be initialized in router (see internal/routes/router.go)
	logging.Info("Prometheus metrics will be initialized during router setup")

	// Connect to DB with GORM
	host := os.Getenv("PG_HOST")
	port := os.Getenv("PG_PORT")
	user := os.Getenv("PG_USER")
	dbname := os.Getenv("PG_DB")
	password := os.Getenv("PG_PASSWORD")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	if _, err := db.InitPostgresORM(dsn); err != nil {
		logging.Error("Failed to connect to Postgres (GORM)", "error", err.Error())
		log.Fatalf("❌ Failed to connect to Postgres (GORM): %v", err)
	}
	logging.Info("Connected to Postgres (GORM)")

	upSince := time.Now()

	// Initialize router with Chi
	// Note: metricsReg is created in RegisterRoutes and applied as global middleware
	router := routes.RegisterRoutes(upSince)

	// Setup metrics endpoint outside of Chi router
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", router) // Mount Chi router at root
	logging.Info("Prometheus metrics endpoint registered at /metrics")

	logging.Info("Server starting",
		"port", 8080,
		"environment", appEnv,
	)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
