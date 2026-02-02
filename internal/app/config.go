package app

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables
type Config struct {
	AppEnv    string
	Debug     bool
	Port      string
	PG        PGConfig
	Redis     RedisConfig
	InfFlight InfFlightConfig
	Admin     AdminConfig
}

// PGConfig holds PostgreSQL connection configuration
type PGConfig struct {
	Host     string
	Port     string
	User     string
	DB       string
	Password string
}

// DSN returns the PostgreSQL connection string
func (c PGConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.Host, c.Port, c.User, c.DB, c.Password)
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host        string
	Port        string
	Password    string
	UseRedisCache bool
}

// InfFlightConfig holds Infinite Flight API configuration
type InfFlightConfig struct {
	BaseURL string
	APIKey  string
}

// AdminConfig holds admin-specific configuration
type AdminConfig struct {
	GodMode   string
	JWTSecret string
}

// LoadConfig reads configuration from environment variables
func LoadConfig() Config {
	return Config{
		AppEnv: getEnv("APP_ENV", "local"),
		Debug:  getBoolEnv("DEBUG", false),
		Port:   getEnv("PORT", "8080"),
		PG: PGConfig{
			Host:     getEnv("PG_HOST", "localhost"),
			Port:     getEnv("PG_PORT", "5432"),
			User:     getEnv("PG_USER", "ieuser"),
			DB:       getEnv("PG_DB", "infinite"),
			Password: getEnv("PG_PASSWORD", "iepass"),
		},
		Redis: RedisConfig{
			Host:        getEnv("REDIS_HOST", "localhost"),
			Port:        getEnv("REDIS_PORT", "6379"),
			Password:    getEnv("REDIS_PASSWORD", ""),
			UseRedisCache: getBoolEnv("USE_REDIS_CACHE", true),
		},
		InfFlight: InfFlightConfig{
			BaseURL: getEnv("IF_API_BASE_URL", "https://api.infiniteflight.com/public/v2"),
			APIKey:  getEnv("IF_API_KEY", ""),
		},
		Admin: AdminConfig{
			GodMode:   getEnv("GOD_MODE", ""),
			JWTSecret: getEnv("JWT_SECRET", ""),
		},
	}
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv gets a boolean environment variable with a fallback default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolVal, err := strconv.ParseBool(value)
		if err == nil {
			return boolVal
		}
	}
	return defaultValue
}
