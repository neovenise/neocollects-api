// Command import updates the catalog and its prices once.
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
	"github.com/neovenise/neocollects-api/internal/catalog/importer"
	"github.com/neovenise/neocollects-api/internal/catalog/store"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/database"
)

func main() {
	details := flag.String("details", "new", `"new": only new cards; "all": every card (rarities and prices)`)
	sets := flag.String("sets", "", "comma-separated set ids to import (default: all)")
	concurrency := flag.Int("concurrency", 4, "card detail requests at once")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, *details, *sets, *concurrency); err != nil {
		logger.Error("import failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, details, sets string, concurrency int) error {
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

	res, err := app.CatalogImporter(cfg, store.NewRepository(pool), logger).Run(ctx, opts)
	if err != nil {
		return err
	}
	fmt.Printf("blocks %d, sets %d, cards %d, details %d, prices %d, failed sets %v, failed cards %d\n",
		res.Blocks, res.Sets, res.Cards, res.Details, res.Prices, res.FailedSets, len(res.FailedCards))
	if len(res.FailedSets) > 0 || len(res.FailedCards) > 0 {
		return fmt.Errorf("%d sets and %d cards failed: run again, the import is safe to repeat", len(res.FailedSets), len(res.FailedCards))
	}
	return nil
}
