package app

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/neovenise/neocollects-api/internal/catalog/foils"
	"github.com/neovenise/neocollects-api/internal/catalog/importer"
	"github.com/neovenise/neocollects-api/internal/catalog/refresh"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
	"github.com/neovenise/neocollects-api/internal/sources/tcgdex"
)

// CatalogJob builds the nightly job: import, foils, then pictures.
func CatalogJob(cfg *config.Config, pool *pgxpool.Pool, logger *slog.Logger) (*refresh.Job, error) {
	repo := store.NewRepository(pool)
	malieClient := malie.NewClient(cfg.MalieBaseURL, cfg.MalieLanguage)
	malieImages := importer.NewMalieImages(malieClient, repo)
	imp := importer.New(tcgdex.NewClient(cfg.TCGdexBaseURL, cfg.TCGdexLang), repo, logger).
		WithCardSupplements(malieImages).
		WithSharpImages(importer.SharpImageSources(malieImages, importer.NewSwshPictures(malieClient, repo, logger))).
		WithImageFallbacks(importer.NewEnglishImages(tcgdex.NewClient(cfg.TCGdexBaseURL, cfg.TCGdexFallbackLang)))
	foilImporters := foils.All(foils.NewImporter(malieClient, repo, logger), foils.NewSwshImporter(malieClient, repo, logger))
	job := refresh.New(imp, repo, logger).WithFoils(foilImporters, repo.FoilCount)
	pictures, err := Mirrors(cfg, repo, logger, nil)
	if err != nil {
		return nil, err
	}
	if pictures != nil {
		job.WithMirror(pictures)
	}
	return job, nil
}
