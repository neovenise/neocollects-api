package config

import (
	"errors"
	"net"
	"net/url"
	"os"
)

type Config struct {
	DatabaseURL string

	HTTPAddr string

	TCGdexBaseURL string
	TCGdexLang    string

	ImagesBaseURL string

	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool

	InviteCode string

	ImportEnabled bool

	Debug bool
}

// Load reads and validates the configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: DatabaseURL(),
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
		Debug:       os.Getenv("APP_DEBUG") == "true",

		TCGdexBaseURL: os.Getenv("TCGDEX_BASE_URL"),
		TCGdexLang:    os.Getenv("TCGDEX_LANG"),

		ImportEnabled: os.Getenv("IMPORT_ENABLED") != "false",
		ImagesBaseURL: os.Getenv("IMAGES_BASE_URL"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3UseSSL:    os.Getenv("S3_USE_SSL") == "true",

		InviteCode: os.Getenv("INVITE_CODE"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("neither DATABASE_URL nor DB_PASSWORD is set")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8090"
	}

	if cfg.TCGdexBaseURL == "" {
		cfg.TCGdexBaseURL = "https://api.tcgdex.net/v2"
	}
	if cfg.TCGdexLang == "" {
		cfg.TCGdexLang = "fr"
	}

	if cfg.S3Bucket == "" {
		cfg.S3Bucket = "images"
	}
	if cfg.S3Endpoint != "" && (cfg.S3AccessKey == "" || cfg.S3SecretKey == "") {
		return nil, errors.New("S3_ENDPOINT is set but S3_ACCESS_KEY or S3_SECRET_KEY is not")
	}

	return cfg, nil
}

// HasImageStorage reports whether an image storage is configured.
func (c *Config) HasImageStorage() bool { return c.S3Endpoint != "" }

// DatabaseURL returns DATABASE_URL, else builds it from DB_PASSWORD and the other DB_* variables.
func DatabaseURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return ""
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(envOr("DB_USER", "neocollects"), password),
		Host:     net.JoinHostPort(envOr("DB_HOST", "127.0.0.1"), envOr("DB_PORT", "5433")),
		Path:     "/" + envOr("DB_NAME", "neocollects"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
