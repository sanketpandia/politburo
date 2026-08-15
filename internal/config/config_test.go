package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadUsesRewriteSafeDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("PG_DB", "")
	t.Setenv("JOBS_ENABLED", "")
	t.Setenv("REDIS_HOST", "")
	t.Setenv("REDIS_PORT", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("SIGNED_LINK_SECRET", "")
	t.Setenv("UI_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Port != "8082" {
		t.Fatalf("port = %q, want 8082", cfg.HTTP.Port)
	}
	if len(cfg.HTTP.AllowedOrigins) != 2 || cfg.HTTP.AllowedOrigins[0] != "http://localhost:8081" {
		t.Fatalf("allowed origins = %#v", cfg.HTTP.AllowedOrigins)
	}
	if cfg.Jobs.Enabled {
		t.Fatal("jobs must be disabled by default")
	}
	if cfg.Database.URL == "" {
		t.Fatal("database URL must have a local default")
	}
	if cfg.Redis.Address() != "localhost:6379" || cfg.Redis.DB != 0 {
		t.Fatalf("Redis config = %#v", cfg.Redis)
	}
	if cfg.InfiniteFlight.BaseURL != "https://api.infiniteflight.com/public/v2" {
		t.Fatalf("Infinite Flight base URL = %q", cfg.InfiniteFlight.BaseURL)
	}
	if cfg.InfiniteFlight.RequestTimeout != 15*time.Second {
		t.Fatalf("Infinite Flight request timeout = %s", cfg.InfiniteFlight.RequestTimeout)
	}
	if len(cfg.Auth.SignedLinkSecret) != 32 {
		t.Fatalf("local signed-link secret length = %d, want 32", len(cfg.Auth.SignedLinkSecret))
	}
	if cfg.Auth.UIBaseURL != "http://localhost:8082" {
		t.Fatalf("UI base URL = %q, want http://localhost:8082", cfg.Auth.UIBaseURL)
	}
}

func TestLoadAcceptsCORSOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://one.example, https://two.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.HTTP.AllowedOrigins) != 2 || cfg.HTTP.AllowedOrigins[0] != "https://one.example" || cfg.HTTP.AllowedOrigins[1] != "https://two.example" {
		t.Fatalf("allowed origins = %#v", cfg.HTTP.AllowedOrigins)
	}
}

func TestLoadAcceptsRedisOverrides(t *testing.T) {
	t.Setenv("REDIS_HOST", "cache.internal")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_PING_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Redis.Address() != "cache.internal:6380" || cfg.Redis.Password != "secret" || cfg.Redis.DB != 2 || cfg.Redis.PingTimeout != 2*time.Second {
		t.Fatalf("Redis config = %#v", cfg.Redis)
	}
}

func TestLoadRejectsNegativeRedisDB(t *testing.T) {
	t.Setenv("REDIS_DB", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid Redis DB error")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}

func TestLoadRequiresInfiniteFlightAPIKeyWhenJobsEnabled(t *testing.T) {
	t.Setenv("JOBS_ENABLED", "true")
	t.Setenv("IF_API_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing IF_API_KEY error")
	}
}

func TestLoadAcceptsInfiniteFlightOverrides(t *testing.T) {
	t.Setenv("JOBS_ENABLED", "true")
	t.Setenv("IF_API_KEY", "secret")
	t.Setenv("IF_API_BASE_URL", "https://example.test/public/v2")
	t.Setenv("IF_API_REQUEST_TIMEOUT", "3s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.InfiniteFlight.APIKey != "secret" {
		t.Fatal("Infinite Flight API key was not loaded")
	}
	if cfg.InfiniteFlight.BaseURL != "https://example.test/public/v2" {
		t.Fatalf("Infinite Flight base URL = %q", cfg.InfiniteFlight.BaseURL)
	}
	if cfg.InfiniteFlight.RequestTimeout != 3*time.Second {
		t.Fatalf("Infinite Flight request timeout = %s", cfg.InfiniteFlight.RequestTimeout)
	}
}

func TestLoadReadsMountedSecretFiles(t *testing.T) {
	directory := t.TempDir()
	writeSecret := func(name, value string) string {
		t.Helper()
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			t.Fatalf("write secret: %v", err)
		}
		return path
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_URL_FILE", writeSecret("database-url", "postgres://file-user:file-pass@db:5432/politburo"))
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_PASSWORD_FILE", writeSecret("redis-password", "redis-secret"))
	t.Setenv("JOBS_ENABLED", "true")
	t.Setenv("IF_API_KEY", "")
	t.Setenv("IF_API_KEY_FILE", writeSecret("if-api-key", "if-secret"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.URL != "postgres://file-user:file-pass@db:5432/politburo" {
		t.Fatalf("database URL = %q", cfg.Database.URL)
	}
	if cfg.Redis.Password != "redis-secret" {
		t.Fatal("Redis password was not loaded from its mounted file")
	}
	if cfg.InfiniteFlight.APIKey != "if-secret" {
		t.Fatal("Infinite Flight API key was not loaded from its mounted file")
	}
}

func TestLoadReadsPostgresPasswordFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postgres-password")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PG_PASSWORD", "")
	t.Setenv("PG_PASSWORD_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !strings.Contains(cfg.Database.URL, ":file-secret@") {
		t.Fatalf("database URL did not use mounted password: %q", cfg.Database.URL)
	}
}

func TestLoadRejectsAmbiguousSecretSources(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "environment-secret")
	t.Setenv("REDIS_PASSWORD_FILE", "/run/secrets/redis-password")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("Load() error = %v, want ambiguous secret source error", err)
	}
}

func TestLoadRejectsUnreadableSecretFile(t *testing.T) {
	t.Setenv("IF_API_KEY", "")
	t.Setenv("IF_API_KEY_FILE", filepath.Join(t.TempDir(), "missing"))

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "read IF_API_KEY_FILE") {
		t.Fatalf("Load() error = %v, want secret file read error", err)
	}
}

func TestLoadRequiresSignedLinkSecretOutsideLocal(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SIGNED_LINK_SECRET", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "SIGNED_LINK_SECRET is required") {
		t.Fatalf("Load() error = %v, want required signed-link secret", err)
	}
}

func TestLoadAcceptsSignedLinkSecretAndUIBaseURL(t *testing.T) {
	t.Setenv("SIGNED_LINK_SECRET", "abcdef0123456789abcdef0123456789")
	t.Setenv("UI_BASE_URL", "https://viz.example/")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(cfg.Auth.SignedLinkSecret) != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("signed-link secret = %q", cfg.Auth.SignedLinkSecret)
	}
	if cfg.Auth.UIBaseURL != "https://viz.example" {
		t.Fatalf("UI base URL = %q", cfg.Auth.UIBaseURL)
	}
}

func TestLoadRejectsInvalidUIBaseURL(t *testing.T) {
	t.Setenv("UI_BASE_URL", "not-a-url")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "UI_BASE_URL") {
		t.Fatalf("Load() error = %v, want invalid UI_BASE_URL", err)
	}
}
