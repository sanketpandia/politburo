package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"infinite-experiment/politburo/infra/db"
	"infinite-experiment/politburo/internal/routes"
)

// Vizburo UI Service
// Runs on port 3000 by default
// Serves flight routes visualization with theme support
// Shares Redis cache and PostgreSQL with Politburo API for distributed load
func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Connect to DB with GORM
	host := os.Getenv("PG_HOST")
	port := os.Getenv("PG_PORT")
	user := os.Getenv("PG_USER")
	dbname := os.Getenv("PG_DB")
	password := os.Getenv("PG_PASSWORD")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	if _, err := db.InitPostgresORM(dsn); err != nil {
		log.Fatalf("❌ Failed to connect to Postgres: %v", err)
	}
	log.Println("✅ Connected to Postgres!")

	upSince := time.Now()

	// Initialize router with Chi (same router as API, includes UI and API routes)
	router := routes.RegisterRoutes(upSince)

	// Get port from environment or use default 3000
	vizburoPort := os.Getenv("VIZBURO_PORT")
	if vizburoPort == "" {
		vizburoPort = "3000"
	}

	listenAddr := ":" + vizburoPort
	log.Println("🚀 Vizburo UI Service starting on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, router))
}
