package app

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/neovenise/neocollects-api/internal/catalog/importer"
	"github.com/neovenise/neocollects-api/internal/catalog/refresh"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/sources/tcgdex"
)

// CatalogJob builds the nightly catalog import.
func CatalogJob(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) *refresh.Job {
	repo := store.NewRepository(pool)
	return refresh.New(CatalogImporter(cfg, repo, logger), repo, logger)
}

// CatalogImporter builds the importer of blocks, sets, cards and prices.
func CatalogImporter(cfg *config.Config, repo *store.Repository, logger *slog.Logger) *importer.Importer {
	return importer.New(tcgdex.NewClient(cfg.TCGdexBaseURL, cfg.TCGdexLang), repo, logger)
}
