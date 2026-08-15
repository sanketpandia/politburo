package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment    string
	HTTP           HTTP
	Database       Database
	Redis          Redis
	Jobs           Jobs
	InfiniteFlight InfiniteFlight
	Auth           Auth
}

type Auth struct {
	SignedLinkSecret []byte
	UIBaseURL        string
}

type HTTP struct {
	Port            string
	AllowedOrigins  []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type Database struct {
	URL         string
	PingTimeout time.Duration
}

type Redis struct {
	Host         string
	Port         string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingTimeout  time.Duration
}

func (r Redis) Address() string {
	return net.JoinHostPort(r.Host, r.Port)
}

type Jobs struct {
	Enabled bool
}

type InfiniteFlight struct {
	BaseURL        string
	APIKey         string
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	databaseURLOverride, err := secret("DATABASE_URL", "")
	if err != nil {
		return Config{}, err
	}
	postgresPassword, err := secret("PG_PASSWORD", "iepass")
	if err != nil {
		return Config{}, err
	}
	redisPassword, err := secret("REDIS_PASSWORD", "")
	if err != nil {
		return Config{}, err
	}
	infiniteFlightAPIKey, err := secret("IF_API_KEY", "")
	if err != nil {
		return Config{}, err
	}
	signedLinkSecret, err := secret("SIGNED_LINK_SECRET", "")
	if err != nil {
		return Config{}, err
	}

	environment := env("APP_ENV", "local")
	parsedSecret, err := parseSignedLinkSecret(signedLinkSecret, environment)
	if err != nil {
		return Config{}, err
	}
	uiBaseURL := strings.TrimRight(env("UI_BASE_URL", "http://localhost:8082"), "/")
	parsedUI, err := url.ParseRequestURI(uiBaseURL)
	if err != nil || parsedUI.Scheme == "" || parsedUI.Host == "" {
		return Config{}, fmt.Errorf("UI_BASE_URL must be an absolute URL")
	}

	cfg := Config{
		Environment: environment,
		HTTP: HTTP{
			Port:            env("PORT", "8082"),
			AllowedOrigins:  csv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:8081", "http://127.0.0.1:8081"}),
			ReadTimeout:     duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: duration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: Database{
			URL:         databaseURL(databaseURLOverride, postgresPassword),
			PingTimeout: duration("DB_PING_TIMEOUT", 5*time.Second),
		},
		Redis: Redis{
			Host:         env("REDIS_HOST", "localhost"),
			Port:         env("REDIS_PORT", "6379"),
			Password:     redisPassword,
			DB:           integer("REDIS_DB", 0),
			DialTimeout:  duration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  duration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: duration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			PingTimeout:  duration("REDIS_PING_TIMEOUT", 5*time.Second),
		},
		Jobs: Jobs{Enabled: boolean("JOBS_ENABLED", false)},
		InfiniteFlight: InfiniteFlight{
			BaseURL:        env("IF_API_BASE_URL", "https://api.infiniteflight.com/public/v2"),
			APIKey:         infiniteFlightAPIKey,
			RequestTimeout: duration("IF_API_REQUEST_TIMEOUT", 15*time.Second),
		},
		Auth: Auth{
			SignedLinkSecret: parsedSecret,
			UIBaseURL:        uiBaseURL,
		},
	}

	if _, err := strconv.Atoi(cfg.HTTP.Port); err != nil {
		return Config{}, fmt.Errorf("PORT must be numeric: %w", err)
	}
	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("database URL is empty")
	}
	if _, err := strconv.Atoi(cfg.Redis.Port); err != nil {
		return Config{}, fmt.Errorf("REDIS_PORT must be numeric: %w", err)
	}
	if cfg.Redis.DB < 0 {
		return Config{}, fmt.Errorf("REDIS_DB must be non-negative")
	}
	baseURL, err := url.ParseRequestURI(cfg.InfiniteFlight.BaseURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return Config{}, fmt.Errorf("IF_API_BASE_URL must be an absolute URL")
	}
	if cfg.Jobs.Enabled && cfg.InfiniteFlight.APIKey == "" {
		return Config{}, fmt.Errorf("IF_API_KEY is required when JOBS_ENABLED=true")
	}
	return cfg, nil
}

// localSignedLinkSecretHex is a documented 32-byte development key.
const localSignedLinkSecretHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func parseSignedLinkSecret(raw, environment string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if environment != "local" {
			return nil, fmt.Errorf("SIGNED_LINK_SECRET is required")
		}
		raw = localSignedLinkSecretHex
	}
	if len(raw) == 64 {
		decoded, err := hex.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("SIGNED_LINK_SECRET must be 32 bytes or 64 hex characters: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("SIGNED_LINK_SECRET must be 32 bytes or 64 hex characters")
		}
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, fmt.Errorf("SIGNED_LINK_SECRET must be 32 bytes or 64 hex characters")
}

func databaseURL(override, password string) string {
	if override != "" {
		return override
	}
	credentials := url.UserPassword(env("PG_USER", "ieuser"), password)
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable",
		credentials.String(),
		env("PG_HOST", "localhost"),
		env("PG_PORT", "5432"),
		env("PG_DB", "politburo_next"),
	)
}

// secret reads a sensitive setting from KEY or from the file named by
// KEY_FILE. File-backed secrets avoid exposing values in container metadata.
func secret(key, fallback string) (string, error) {
	value := os.Getenv(key)
	path := os.Getenv(key + "_FILE")
	if value != "" && path != "" {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", key, key)
	}
	if path == "" {
		if value == "" {
			return fallback, nil
		}
		return value, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", key, err)
	}
	return strings.TrimRight(string(contents), "\r\n"), nil
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

func integer(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
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

func csv(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
