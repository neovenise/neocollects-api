package mirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

const logoVersion = "1"

const (
	logoWidth     = 480
	logoQuality   = 85
	symbolWidth   = 128
	symbolQuality = 90
)

const LogoSourceBase = "https://pokecardex.b-cdn.net/assets/images/logos/"

type LogoStore interface {
	SetsWithoutLogoCopy(ctx context.Context) ([]catalog.LogoCandidate, error)
	SetLogoKey(ctx context.Context, setID, key string) error
	SetSymbolKey(ctx context.Context, setID, key string) error
}

type Logos struct {
	db      LogoStore
	fetcher Fetcher
	store   Storage
	logger  *slog.Logger
}

func NewLogos(db LogoStore, fetcher Fetcher, store Storage, logger *slog.Logger) *Logos {
	return &Logos{db: db, fetcher: fetcher, store: store, logger: logger}
}

// Run copies the missing set logos and symbols.
func (m *Logos) Run(ctx context.Context, _ int) (Result, error) {
	var res Result
	sets, err := m.db.SetsWithoutLogoCopy(ctx)
	if err != nil {
		return res, err
	}
	for _, set := range sets {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if set.NeedsLogo {
			m.copyOne(ctx, &res, set.ID, "logo", m.logoSources(set), logoWidth, logoQuality, m.db.SetLogoKey)
		}
		if set.NeedsSymbol {
			m.copyOne(ctx, &res, set.ID, "symbol", []string{set.SymbolSource + ".png"}, symbolWidth, symbolQuality, m.db.SetSymbolKey)
		}
	}
	return res, nil
}

// logoSources lists where to look for a set's logo, best first.
func (m *Logos) logoSources(set catalog.LogoCandidate) []string {
	var sources []string
	if set.LogoSource != "" {
		sources = append(sources, set.LogoSource+".webp")
	}
	if set.Code != "" {
		sources = append(sources, LogoSourceBase+strings.ToUpper(set.Code)+".png")
	}
	return sources
}

// copyOne stores the first source that can be fetched and records its key.
func (m *Logos) copyOne(ctx context.Context, res *Result, setID, kind string, sources []string, width, quality int,
	record func(context.Context, string, string) error) {
	for _, source := range sources {
		if source == ".png" || source == ".webp" {
			continue
		}
		data, err := m.fetcher.Fetch(ctx, source)
		if err != nil {
			continue
		}
		if err := m.copy(ctx, setID, kind, source, data, width, quality, record); err != nil {
			m.logger.Warn("could not copy a set picture", "set", setID, "kind", kind, "error", err)
			res.Failed = append(res.Failed, setID+" "+kind)
			return
		}
		res.Copied++
		return
	}
	res.UpToDate++
}

func (m *Logos) copy(ctx context.Context, setID, kind, source string, data []byte, width, quality int,
	record func(context.Context, string, string) error) error {
	decoded, err := storage.Decode(data)
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	file, err := storage.EncodeScaled(decoded.Image, width, quality)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(logoVersion + "|" + source))
	key := fmt.Sprintf("sets/%s.%s.%s.webp", safeKey(setID), kind, hex.EncodeToString(sum[:])[:8])
	if err := m.store.Put(ctx, key, file); err != nil {
		return err
	}
	return record(ctx, setID, key)
}
