package config

import "testing"

func TestLoadUsesRewriteSafeDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("PG_DB", "")
	t.Setenv("JOBS_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Port != "8082" {
		t.Fatalf("port = %q, want 8082", cfg.HTTP.Port)
	}
	if cfg.Jobs.Enabled {
		t.Fatal("jobs must be disabled by default")
	}
	if cfg.Database.URL == "" {
		t.Fatal("database URL must have a local default")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}
