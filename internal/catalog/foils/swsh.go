package foils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/catalog/tcglive"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

var plainRarities = map[string]bool{"Commune": true, "Peu Commune": true, "Rare": true}

func swshLook(rarity string) string {
	switch rarity {
	case "Holo Rare V", "Holo Rare VMAX", "Holo Rare VSTAR", "Shiny rare V", "Shiny rare VMAX", "Ultra Rare", "Dresseur Full Art":
		return "fullArt"
	default:
		return "holo"
	}
}

type Prober interface {
	Exists(ctx context.Context, rawURL string) (bool, error)
}

type SwshStore interface {
	FoillessCards(ctx context.Context, setIDs []string) ([]catalog.FoilCandidate, error)
	UpsertFoil(ctx context.Context, f catalog.Foil) error
}

type SwshImporter struct {
	prober      Prober
	store       SwshStore
	logger      *slog.Logger
	concurrency int
}

func NewSwshImporter(prober Prober, store SwshStore, logger *slog.Logger) *SwshImporter {
	return &SwshImporter{prober: prober, store: store, logger: logger, concurrency: 4}
}

// Run imports the Sword & Shield foils from the game.
func (i *SwshImporter) Run(ctx context.Context) (Result, error) {
	var res Result
	setIDs := make([]string, 0, len(tcglive.SwshSets))
	for id := range tcglive.SwshSets {
		setIDs = append(setIDs, id)
	}
	candidates, err := i.store.FoillessCards(ctx, setIDs)
	if err != nil {
		return res, fmt.Errorf("swsh foil import: %w", err)
	}

	var (
		mu       sync.Mutex
		sets     = map[string]bool{}
		failed   = map[string]bool{}
		wg       sync.WaitGroup
		work     = make(chan catalog.FoilCandidate)
		firstErr error
	)
	for w := 0; w < i.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for card := range work {
				foil, found, err := i.find(ctx, card)
				if err == nil && found {
					err = i.store.UpsertFoil(ctx, foil)
				}
				mu.Lock()
				switch {
				case err != nil && ctx.Err() != nil:
					firstErr = ctx.Err()
				case err != nil:
					i.logger.Error("swsh foil import failed for a card", "card", card.CardID, "error", err)
					failed[card.SetID] = true
				case found:
					sets[card.SetID] = true
					res.Foils++
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for _, card := range candidates {
		if plainRarities[card.Rarity] {
			continue
		}
		select {
		case work <- card:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()

	if firstErr == nil {
		firstErr = ctx.Err()
	}
	res.Sets = len(sets)
	for id := range failed {
		res.FailedSets = append(res.FailedSets, id)
	}
	sort.Strings(res.FailedSets)
	if firstErr != nil {
		return res, firstErr
	}
	i.logger.Info("swsh foils imported", "foils", res.Foils, "sets", res.Sets)
	return res, nil
}

func (i *SwshImporter) find(ctx context.Context, card catalog.FoilCandidate) (catalog.Foil, bool, error) {
	set, ok := tcglive.SwshSets[card.SetID]
	if !ok {
		return catalog.Foil{}, false, nil
	}
	number, ok := tcglive.GameNumber(card.LocalID, set.Offset)
	if !ok {
		return catalog.Foil{}, false, nil
	}
	for _, lang := range tcglive.Languages {
		front, mask, etch := malie.PictureURLs(lang, set.Code, number)
		hasMask, err := i.prober.Exists(ctx, mask)
		if err != nil {
			return catalog.Foil{}, false, err
		}
		if !hasMask {
			continue
		}
		hasFront, err := i.prober.Exists(ctx, front)
		if err != nil {
			return catalog.Foil{}, false, err
		}
		if !hasFront {
			continue
		}
		hasEtch, err := i.prober.Exists(ctx, etch)
		if err != nil {
			return catalog.Foil{}, false, err
		}
		if !hasEtch {
			etch = ""
		}
		sum := sha256.Sum256([]byte(front + "|" + mask + "|" + etch))
		return catalog.Foil{
			CardID: card.CardID, Look: swshLook(card.Rarity),
			FrontURL: front, MaskURL: mask, EtchURL: etch, SourceHash: hex.EncodeToString(sum[:16]),
		}, true, nil
	}
	return catalog.Foil{}, false, nil
}
