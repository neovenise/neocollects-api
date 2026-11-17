package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when DATABASE_URL is not set")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("APP_DEBUG", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8090" {
		t.Errorf("HTTPAddr = %q, want :8090", cfg.HTTPAddr)
	}
	if cfg.Debug {
		t.Error("Debug must be off by default")
	}
	if cfg.TCGdexBaseURL != "https://api.tcgdex.net/v2" || cfg.TCGdexLang != "fr" {
		t.Errorf("TCGdex defaults = %q, %q", cfg.TCGdexBaseURL, cfg.TCGdexLang)
	}
	if !cfg.ImportEnabled {
		t.Error("the import must be on by default")
	}
	if cfg.TCGdexFallbackLang != "en" {
		t.Errorf("fallback language = %q, want en", cfg.TCGdexFallbackLang)
	}
}

func TestImportCanBeSwitchedOff(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("IMPORT_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ImportEnabled {
		t.Error("IMPORT_ENABLED=false must switch the import off")
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("APP_DEBUG", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" || !cfg.Debug {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
