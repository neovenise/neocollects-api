// Command import updates the catalog and its pictures once.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/neovenise/neocollects-api/internal/app"
	"github.com/neovenise/neocollects-api/internal/catalog/foils"
	"github.com/neovenise/neocollects-api/internal/catalog/importer"
	"github.com/neovenise/neocollects-api/internal/catalog/mirror"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/database"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
	"github.com/neovenise/neocollects-api/internal/sources/tcgdex"
)

var archiveSource string

func main() {
	details := flag.String("details", "new", `"new": only new cards; "all": every card (rarities and prices)`)
	sets := flag.String("sets", "", "comma-separated set ids to import (default: all)")
	concurrency := flag.Int("concurrency", 4, "card detail requests at once")
	foilsOnly := flag.Bool("foils-only", false, "import only the foils (malie.io), not the catalog")
	withFoils := flag.Bool("foils", false, "import the foils (malie.io) after the catalog")
	withImages := flag.Bool("images", false, "copy the card pictures to the image storage after the catalog")
	imagesOnly := flag.Bool("images-only", false, "only copy the card pictures to the image storage, not the catalog")
	imageConcurrency := flag.Int("image-concurrency", 4, "pictures copied at once")
	imagesFrom := flag.String("images-from", "", "copy the pictures of this folder to the image storage, named after card ids (\"30th-c-001.png\"), for cards no source has a picture of")
	logosOnly := flag.Bool("logos-only", false, "only copy the missing set logos and symbols to the image storage")
	archiveDir := flag.String("malie-archive", "", "read malie.io's data and pictures from this archive (go run ./cmd/malie-archive create) instead of the CDN")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *archiveDir != "" {
		archiveSource = *archiveDir
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, *details, *sets, *concurrency, *foilsOnly, *withFoils, *withImages, *imagesOnly, *imageConcurrency, *logosOnly, *imagesFrom); err != nil {
		logger.Error("import failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, details, sets string, concurrency int, foilsOnly, withFoils, withImages, imagesOnly bool, imageConcurrency int, logosOnly bool, imagesFrom string) error {
	opts := importer.Options{Concurrency: concurrency}
	switch details {
	case "new":
		opts.Details = importer.DetailsNew
	case "all":
		opts.Details = importer.DetailsAll
	default:
		return fmt.Errorf(`-details must be "new" or "all", got %q`, details)
	}
	if sets != "" {
		opts.SetIDs = strings.Split(sets, ",")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		return err
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Run a single step when one is asked for.
	repo := store.NewRepository(pool)
	if foilsOnly {
		return runFoils(ctx, logger, cfg, repo)
	}
	if imagesFrom != "" {
		return runFolder(ctx, logger, cfg, repo, imagesFrom)
	}
	if logosOnly {
		return runLogos(ctx, logger, cfg, repo)
	}
	if imagesOnly {
		return runImages(ctx, logger, cfg, repo, imageConcurrency)
	}

	// Build the importer with every picture source.
	malieImages := importer.NewMalieImages(malieSource(cfg), repo)
	englishImages := importer.NewEnglishImages(tcgdex.NewClient(cfg.TCGdexBaseURL, cfg.TCGdexFallbackLang))

	swshPictures := importer.NewSwshPictures(malie.NewClient(cfg.MalieBaseURL, cfg.MalieLanguage), repo, logger)
	importer := importer.New(tcgdex.NewClient(cfg.TCGdexBaseURL, cfg.TCGdexLang), repo, logger).
		WithCardSupplements(malieImages).
		WithSharpImages(importer.SharpImageSources(malieImages, swshPictures)).
		WithImageFallbacks(englishImages)
	res, err := importer.Run(ctx, opts)
	if err != nil {
		return err
	}

	if withFoils {
		if err := runFoils(ctx, logger, cfg, repo); err != nil {
			return err
		}
	}
	if withImages {
		if err := runImages(ctx, logger, cfg, repo, imageConcurrency); err != nil {
			return err
		}
	}
	fmt.Printf("blocks %d, sets %d, cards %d, details %d, prices %d, cards added %d, sharp pictures %d, images from the fallback %d, failed sets %v, failed cards %d\n",
		res.Blocks, res.Sets, res.Cards, res.Details, res.Prices, res.CardsAdded, res.SharpImages, res.ImagesFilled, res.FailedSets, len(res.FailedCards))
	if len(res.FailedSets) > 0 || len(res.FailedCards) > 0 {
		return fmt.Errorf("%d sets and %d cards failed: run again, the import is safe to repeat", len(res.FailedSets), len(res.FailedCards))
	}
	return nil
}

// runFolder copies the pictures of a local folder.
func runFolder(ctx context.Context, logger *slog.Logger, cfg *config.Config, repo *store.Repository, dir string) error {
	store, err := app.PictureStore(cfg)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("no image storage: set S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}
	res, err := mirror.NewFolder(repo, store, logger).Run(ctx, dir)
	fmt.Printf("folder %s: copied %d, up to date %d, unknown card %v, failed %v\n", dir, res.Copied, res.UpToDate, res.Unknown, res.Failed)
	if err == nil && (len(res.Unknown) > 0 || len(res.Failed) > 0) {
		err = fmt.Errorf("%d unknown and %d failed", len(res.Unknown), len(res.Failed))
	}
	return err
}

// runLogos copies the missing set logos.
func runLogos(ctx context.Context, logger *slog.Logger, cfg *config.Config, repo *store.Repository) error {
	mirror, err := app.Logos(cfg, repo, logger)
	if err != nil {
		return err
	}
	if mirror == nil {
		return fmt.Errorf("no image storage: set S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}
	res, err := mirror.Run(ctx, 1)
	fmt.Printf("set pictures: copied %d, none found %d, failed %v\n", res.Copied, res.UpToDate, res.Failed)
	return err
}

// runImages copies the card and foil pictures.
func runImages(ctx context.Context, logger *slog.Logger, cfg *config.Config, repo *store.Repository, concurrency int) error {
	mirror, err := app.Mirrors(cfg, repo, logger, archiveOf(cfg))
	if err != nil {
		return err
	}
	if mirror == nil {
		return fmt.Errorf("no image storage: set S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}
	res, err := mirror.Run(ctx, concurrency)
	fmt.Printf("pictures: copied %d, up to date %d, failed %d\n", res.Copied, res.UpToDate, len(res.Failed))
	if err != nil {
		return err
	}
	if len(res.Failed) > 0 {
		return fmt.Errorf("%d pictures failed: run again, the copy is safe to repeat", len(res.Failed))
	}
	return nil
}

// runFoils imports every foil.
func runFoils(ctx context.Context, logger *slog.Logger, cfg *config.Config, repo *store.Repository) error {

	importer := foils.All(
		foils.NewImporter(malieSource(cfg), repo, logger),
		foils.NewSwshImporter(malie.NewClient(cfg.MalieBaseURL, cfg.MalieLanguage), repo, logger))
	res, err := importer.Run(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("foils: sets %d, foils %d, unmatched %d, failed sets %v\n", res.Sets, res.Foils, res.Unmatched, res.FailedSets)
	if len(res.FailedSets) > 0 {
		return fmt.Errorf("%d sets failed: run again, the import is safe to repeat", len(res.FailedSets))
	}
	return nil
}

func archiveOf(cfg *config.Config) *malie.Archive {
	if archiveSource == "" {
		return nil
	}
	return malie.OpenArchive(archiveSource, cfg.MalieLanguage)
}

// malieSource prefers the local archive when one is configured.
func malieSource(cfg *config.Config) malie.Source {
	if a := archiveOf(cfg); a != nil {
		return a
	}
	return malie.NewClient(cfg.MalieBaseURL, cfg.MalieLanguage)
}
