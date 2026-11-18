package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/workpool"
	"github.com/neovenise/neocollects-api/internal/sources/tcgdex"
)

type Source interface {
	Series(ctx context.Context) ([]tcgdex.Serie, error)
	SetRefs(ctx context.Context) ([]tcgdex.SetRef, error)
	Set(ctx context.Context, id string) (tcgdex.Set, error)
	Card(ctx context.Context, id string) (tcgdex.Card, error)
}

type Store interface {
	UpsertBlock(ctx context.Context, b catalog.Block) error
	EnsureBlock(ctx context.Context, b catalog.Block) error
	UpsertSet(ctx context.Context, s catalog.Set) error
	UpsertCard(ctx context.Context, c catalog.Card) error
	UpsertPrice(ctx context.Context, p catalog.Price) error
	CardIDs(ctx context.Context, setID string) (map[string]bool, error)
	SetCount(ctx context.Context) (int, error)
}

type Details int

const (
	DetailsNew Details = iota

	DetailsAll
)

type Options struct {
	Details Details

	SetIDs []string

	Concurrency int
}

type Result struct {
	Blocks  int
	Sets    int
	Cards   int
	Details int
	Prices  int

	FailedSets  []string
	FailedCards []string
}

type Importer struct {
	source Source
	store  Store
	logger *slog.Logger

	now func() time.Time
}

func New(source Source, store Store, logger *slog.Logger) *Importer {
	return &Importer{source: source, store: store, logger: logger, now: time.Now}
}

// Run imports blocks, sets, cards and prices.
func (i *Importer) Run(ctx context.Context, opts Options) (Result, error) {
	var res Result
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	series, err := i.source.Series(ctx)
	if err != nil {
		return res, fmt.Errorf("catalog import: list blocks: %w", err)
	}

	for idx, s := range series {
		if err := i.store.UpsertBlock(ctx, catalog.Block{ID: s.ID, Name: s.Name, Position: len(series) - idx, Digital: catalog.IsDigitalBlock(s.ID)}); err != nil {
			return res, err
		}
		res.Blocks++
	}

	setIDs := opts.SetIDs
	if len(setIDs) == 0 {
		refs, err := i.source.SetRefs(ctx)
		if err != nil {
			return res, fmt.Errorf("catalog import: list sets: %w", err)
		}
		for _, ref := range refs {
			setIDs = append(setIDs, ref.ID)
		}
	}

	for _, id := range setIDs {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := i.importSet(ctx, id, opts, len(series)+1, &res); err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			i.logger.Error("set import failed", "set", id, "error", err)
			res.FailedSets = append(res.FailedSets, id)
		}
	}
	return res, nil
}

func (i *Importer) importSet(ctx context.Context, id string, opts Options, orphanPosition int, res *Result) error {
	set, err := i.source.Set(ctx, id)
	if err != nil {
		return err
	}

	if err := i.store.EnsureBlock(ctx, catalog.Block{ID: set.Serie.ID, Name: set.Serie.Name, Position: orphanPosition, Digital: catalog.IsDigitalBlock(set.Serie.ID)}); err != nil {
		return err
	}
	var release *time.Time
	if t, err := time.Parse(time.DateOnly, set.ReleaseDate); err == nil {
		release = &t
	}
	if err := i.store.UpsertSet(ctx, catalog.Set{
		ID: set.ID, Code: set.Abbreviation.Official, Name: set.Name, BlockID: set.Serie.ID,
		OfficialCount: set.CardCount.Official, TotalCount: set.CardCount.Total, ReleaseDate: release,
		LogoSourceURL: set.Logo, SymbolSourceURL: set.Symbol,
	}); err != nil {
		return err
	}
	res.Sets++

	known, err := i.store.CardIDs(ctx, set.ID)
	if err != nil {
		return err
	}
	var toDetail []string
	for _, c := range set.Cards {
		if err := i.store.UpsertCard(ctx, catalog.Card{
			ID: c.ID, SetID: set.ID, LocalID: c.LocalID, Name: c.Name, SourceImageURL: c.Image,
		}); err != nil {
			return err
		}
		res.Cards++
		if opts.Details == DetailsAll || !known[c.ID] {
			toDetail = append(toDetail, c.ID)
		}
	}

	failed := i.importDetails(ctx, set.ID, toDetail, opts.Concurrency, res)
	res.FailedCards = append(res.FailedCards, failed...)
	i.logger.Info("set imported", "set", set.ID, "cards", len(set.Cards), "details", len(toDetail), "failed", len(failed))
	return nil
}

func (i *Importer) importDetails(ctx context.Context, setID string, ids []string, concurrency int, res *Result) []string {
	var (
		mu     sync.Mutex
		failed []string
	)
	_ = workpool.ForEach(ctx, ids, concurrency, func(id string) {
		priced, err := i.importCardDetail(ctx, setID, id)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				i.logger.Warn("card detail failed", "card", id, "error", err)
				failed = append(failed, id)
			}
			return
		}
		res.Details++
		if priced {
			res.Prices++
		}
	})
	return failed
}

func (i *Importer) importCardDetail(ctx context.Context, setID, id string) (priced bool, err error) {
	card, err := i.source.Card(ctx, id)
	if err != nil {
		return false, err
	}
	if err := i.store.UpsertCard(ctx, catalog.Card{
		ID: card.ID, SetID: setID, LocalID: card.LocalID, Name: card.Name, Rarity: card.Rarity, SourceImageURL: card.Image,
		Category: card.Category, Types: card.Types, HP: card.HP, Stage: card.Stage, TrainerType: card.TrainerType,
		RegulationMark: card.RegulationMark, Illustrator: card.Illustrator,
	}); err != nil {
		return false, err
	}
	cents, ok := card.TrendCents()
	if !ok {
		return false, nil
	}
	updated := card.PriceUpdatedAt()
	if updated.IsZero() {
		updated = i.now().UTC()
	}
	day, week, month := card.AverageCents()
	return true, i.store.UpsertPrice(ctx, catalog.Price{
		CardID: card.ID, Cents: cents, Currency: "EUR", UpdatedAt: updated,
		Avg1Cents: day, Avg7Cents: week, Avg30Cents: month, ProductID: card.ProductID(),
	})
}
