package foils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/catalog/tcglive"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

// Look picks the foil look from foil type and rarity.
func Look(foilType, rarity string) string {
	switch {
	case foilType == "SV_ULTRA" && rarity == "SPECIAL_ILLUSTRATION_RARE":
		return "sir"
	case (foilType == "SV_ULTRA" || foilType == "SUN_PILLAR") && rarity == "ULTRA_RARE":
		return "fullArt"
	case foilType == "SUN_PILLAR":
		return "pillar"
	case foilType == "SV_ULTRA":
		return "ex"
	case foilType == "FLAT_SILVER":
		return "reverse"
	case foilType == "ULTRA_GOLD_RAINBOW":
		return "gold"
	default:
		return "holo"
	}
}

type Source interface {
	Index(ctx context.Context) (map[string]malie.IndexEntry, error)
	Set(ctx context.Context, path string) ([]malie.Card, error)
}

type Store interface {
	SetsByCode(ctx context.Context) (map[string][]string, error)
	CardIDsByNumber(ctx context.Context, setID string) (map[int]string, error)
	UpsertFoil(ctx context.Context, f catalog.Foil) error
	FoilCount(ctx context.Context) (int, error)
}

type Result struct {
	Sets int

	Foils int

	Unmatched  int
	FailedSets []string
}

type Importer struct {
	source Source
	store  Store
	logger *slog.Logger
}

func NewImporter(source Source, store Store, logger *slog.Logger) *Importer {
	return &Importer{source: source, store: store, logger: logger}
}

// Run imports the foils of every known set.
func (i *Importer) Run(ctx context.Context) (Result, error) {
	var res Result
	index, err := i.source.Index(ctx)
	if err != nil {
		return res, fmt.Errorf("foil import: read the index: %w", err)
	}
	setsByCode, err := i.store.SetsByCode(ctx)
	if err != nil {
		return res, err
	}

	linked := make(map[string]string, len(tcglive.ExportSets))
	for setID, key := range tcglive.ExportSets {
		linked[key] = setID
	}

	for key, entry := range index {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		setID, ok := linked[key]
		if !ok {
			code := ourCode(entry.Abbr)
			sets := setsByCode[code]
			if code == "" || len(sets) == 0 {
				continue
			}
			if len(sets) > 1 {

				i.logger.Warn("foil import: ambiguous set code, skipped", "code", code, "sets", sets)
				continue
			}
			setID = sets[0]
		}
		if err := i.importSet(ctx, setID, entry, &res); err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			i.logger.Error("foil import failed for a set", "set", setID, "error", err)
			res.FailedSets = append(res.FailedSets, setID)
		}
	}
	return res, nil
}

func ourCode(malieAbbr string) string {
	abbr := strings.ToUpper(strings.TrimSpace(malieAbbr))
	for ours, theirs := range tcglive.ExportCodes {
		if abbr == theirs {
			return ours
		}
	}
	return abbr
}

func (i *Importer) importSet(ctx context.Context, setID string, entry malie.IndexEntry, res *Result) error {
	cards, err := i.source.Set(ctx, entry.Path)
	if err != nil {
		return err
	}
	ids, err := i.store.CardIDsByNumber(ctx, setID)
	if err != nil {
		return err
	}
	res.Sets++
	found := 0
	for _, card := range cards {
		if card.IsReverseVariant() || card.CollectorNumber.Numeric == nil {
			continue
		}
		front, mask, etch, ok := card.FoilImages()
		if !ok {
			continue
		}
		cardID, known := ids[*card.CollectorNumber.Numeric]
		if !known {
			res.Unmatched++
			continue
		}
		sum := sha256.Sum256([]byte(front + "|" + mask + "|" + etch))
		if err := i.store.UpsertFoil(ctx, catalog.Foil{
			CardID: cardID, Look: Look(card.FoilType(), card.RarityDesignation()),
			FrontURL: front, MaskURL: mask, EtchURL: etch, SourceHash: hex.EncodeToString(sum[:16]),
		}); err != nil {
			return err
		}
		res.Foils++
		found++
	}
	i.logger.Info("foils imported", "set", setID, "foils", found)
	return nil
}
