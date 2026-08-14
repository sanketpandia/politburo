package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment string
	HTTP        HTTP
	Database    Database
	Jobs        Jobs
}

type HTTP struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type Database struct {
	URL         string
	PingTimeout time.Duration
}

type Jobs struct {
	Enabled bool
}

func Load() (Config, error) {
	cfg := Config{
		Environment: env("APP_ENV", "local"),
		HTTP: HTTP{
			Port:            env("PORT", "8082"),
			ReadTimeout:     duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: duration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: Database{
			URL:         databaseURL(),
			PingTimeout: duration("DB_PING_TIMEOUT", 5*time.Second),
		},
		Jobs: Jobs{Enabled: boolean("JOBS_ENABLED", false)},
	}

	if _, err := strconv.Atoi(cfg.HTTP.Port); err != nil {
		return Config{}, fmt.Errorf("PORT must be numeric: %w", err)
	}
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("database URL is empty")
	}
	return cfg, nil
}

func databaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env("PG_USER", "ieuser"),
		env("PG_PASSWORD", "iepass"),
		env("PG_HOST", "localhost"),
		env("PG_PORT", "5432"),
		env("PG_DB", "politburo_next"),
	)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func boolean(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func duration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
