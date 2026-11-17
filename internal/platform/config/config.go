package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string

	HTTPAddr string

	TCGdexBaseURL string
	TCGdexLang    string

	TCGdexFallbackLang string

	MalieBaseURL  string
	MalieLanguage string

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
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
		Debug:       os.Getenv("APP_DEBUG") == "true",

		TCGdexBaseURL:      os.Getenv("TCGDEX_BASE_URL"),
		TCGdexLang:         os.Getenv("TCGDEX_LANG"),
		TCGdexFallbackLang: os.Getenv("TCGDEX_FALLBACK_LANG"),

		ImportEnabled: os.Getenv("IMPORT_ENABLED") != "false",
		ImagesBaseURL: os.Getenv("IMAGES_BASE_URL"),
		MalieBaseURL:  os.Getenv("MALIE_BASE_URL"),
		MalieLanguage: os.Getenv("MALIE_LANGUAGE"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3UseSSL:    os.Getenv("S3_USE_SSL") == "true",

		InviteCode: os.Getenv("INVITE_CODE"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
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

	if cfg.TCGdexFallbackLang == "" {
		cfg.TCGdexFallbackLang = "en"
	}
	if cfg.MalieBaseURL == "" {
		cfg.MalieBaseURL = "https://cdn.malie.io/file/malie-io/tcgl/export/"
	}
	if cfg.MalieLanguage == "" {
		cfg.MalieLanguage = "fr-FR"
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
