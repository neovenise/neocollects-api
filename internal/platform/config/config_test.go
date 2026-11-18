package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_PASSWORD", "")

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

func TestDatabaseURLIsBuiltFromItsParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_PASSWORD", "p@ss/word")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_NAME", "")

	got := DatabaseURL()
	want := "postgres://neocollects:p%40ss%2Fword@127.0.0.1:5433/neocollects?sslmode=disable"
	if got != want {
		t.Errorf("DatabaseURL = %q, want %q", got, want)
	}

	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "other")
	if got := DatabaseURL(); got != "postgres://neocollects:p%40ss%2Fword@db:5432/other?sslmode=disable" {
		t.Errorf("DB_* overrides not applied: %q", got)
	}
}

func TestDatabaseURLWinsOverItsParts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://explicit")
	t.Setenv("DB_PASSWORD", "ignored")

	if got := DatabaseURL(); got != "postgres://explicit" {
		t.Errorf("DatabaseURL = %q, want the explicit one", got)
	}
}

func TestLoadAcceptsDBPasswordAlone(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_PASSWORD", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL must be built from DB_PASSWORD")
	}
}
