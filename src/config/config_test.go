package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 7493 {
		t.Fatalf("expected port 7493, got %d", cfg.Server.Port)
	}
	if !cfg.SearXNG.Enabled {
		t.Fatalf("expected SearXNG enabled by default")
	}
	if cfg.Database.Type != "sqlite" {
		t.Fatalf("expected default database type sqlite, got %s", cfg.Database.Type)
	}
	if !cfg.Database.Enabled {
		t.Fatalf("expected default database enabled")
	}
	if cfg.Logs.Storage != "sqlite" {
		t.Fatalf("expected default logs storage sqlite, got %s", cfg.Logs.Storage)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("PORT", "5050")
	os.Setenv("SEARXNG_URL", "http://custom-searxng:8080")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SEARXNG_URL")
	}()

	cfg := LoadConfig()
	if cfg.Server.Port != 5050 {
		t.Fatalf("expected overridden port 5050, got %d", cfg.Server.Port)
	}
	if cfg.SearXNG.URL != "http://custom-searxng:8080" {
		t.Fatalf("expected overridden SearXNG URL, got %s", cfg.SearXNG.URL)
	}

	// Test Vector Dimension overrides
	os.Setenv("VECTOR_DIMENSION", "1536")
	cfgDim := LoadConfig()
	if cfgDim.Database.Postgres.VectorDimension != 1536 {
		t.Fatalf("expected overridden VectorDimension 1536, got %d", cfgDim.Database.Postgres.VectorDimension)
	}
	os.Unsetenv("VECTOR_DIMENSION")
}
