package app

import (
	"log/slog"

	"github.com/neovenise/neocollects-api/internal/catalog/mirror"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/storage"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

var trustedPictureHosts = []string{"assets.tcgdex.net", "cdn.malie.io"}

// Mirror returns the card picture copier, nil without storage.
func Mirror(cfg *config.Config, repo *store.Repository, logger *slog.Logger, archive *malie.Archive) (*mirror.Cards, error) {
	if !cfg.HasImageStorage() {
		return nil, nil
	}
	store, err := storage.NewS3(storage.S3Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}
	fetcher := storage.NewFetcher(trustedPictureHosts...)
	if archive != nil {

		fetcher.WithLocalCopies(archive.Picture).WithSaver(archive.Save)
	}
	return mirror.NewCards(repo, fetcher, store, logger), nil
}

var logoSourceHosts = []string{"assets.tcgdex.net", "pokecardex.b-cdn.net"}

// Logos returns the set logo copier, nil without storage.
func Logos(cfg *config.Config, repo *store.Repository, logger *slog.Logger) (*mirror.Logos, error) {
	if !cfg.HasImageStorage() {
		return nil, nil
	}
	store, err := storage.NewS3(storage.S3Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return mirror.NewLogos(repo, storage.NewFetcher(logoSourceHosts...), store, logger), nil
}

// PictureStore opens the image storage, nil when none is configured.
func PictureStore(cfg *config.Config) (storage.Store, error) {
	if !cfg.HasImageStorage() {
		return nil, nil
	}
	return storage.NewS3(storage.S3Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
}

// Mirrors chains the card, foil and set picture copiers.
func Mirrors(cfg *config.Config, repo *store.Repository, logger *slog.Logger, archive *malie.Archive) (mirror.Runner, error) {
	cards, err := Mirror(cfg, repo, logger, archive)
	if err != nil || cards == nil {
		return nil, err
	}
	store, err := PictureStore(cfg)
	if err != nil {
		return nil, err
	}
	fetcher := storage.NewFetcher(trustedPictureHosts...)
	if archive != nil {
		fetcher.WithLocalCopies(archive.Picture).WithSaver(archive.Save)
	}
	logos := mirror.NewLogos(repo, storage.NewFetcher(logoSourceHosts...), store, logger)
	return mirror.All(cards, mirror.NewFoils(repo, fetcher, store, logger), logos), nil
}
