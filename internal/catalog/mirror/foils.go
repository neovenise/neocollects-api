package mirror

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"sync"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

type FoilStore interface {
	FoilsToMirror(ctx context.Context) ([]catalog.FoilMirrorRow, error)
	SetFoilKeys(ctx context.Context, cardID, frontKey, maskKey, etchKey, hash string) error
}

type Foils struct {
	db      FoilStore
	fetcher Fetcher
	store   Storage
	logger  *slog.Logger
}

func NewFoils(db FoilStore, fetcher Fetcher, store Storage, logger *slog.Logger) *Foils {
	return &Foils{db: db, fetcher: fetcher, store: store, logger: logger}
}

const foilVersion = "1"

// FoilKeys returns the storage keys of a foil's three pictures.
func FoilKeys(cardID, hash string) (front, mask, etch string) {
	safe := safeKey(cardID)
	short := hash
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("foils/%s.%s.%s.front.webp", safe, foilVersion, short),
		fmt.Sprintf("foils/%s.%s.%s.mask.webp", safe, foilVersion, short),
		fmt.Sprintf("foils/%s.%s.%s.etch.webp", safe, foilVersion, short)
}

// Run copies the foil pictures that are missing or stale.
func (m *Foils) Run(ctx context.Context, concurrency int) (Result, error) {
	var res Result
	if concurrency <= 0 {
		concurrency = 4
	}
	foils, err := m.db.FoilsToMirror(ctx)
	if err != nil {
		return res, err
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		slots = make(chan struct{}, concurrency)
	)
	for _, foil := range foils {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return res, ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			err := m.copyFoil(ctx, foil)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				res.Copied++
				if res.Copied%250 == 0 {
					m.logger.Info("foils copied", "done", res.Copied, "to_do", len(foils))
				}
			case errors.Is(err, context.Canceled):
			default:
				m.logger.Warn("could not copy a foil", "card", foil.CardID, "error", err)
				res.Failed = append(res.Failed, foil.CardID)
			}
		}()
	}
	wg.Wait()
	return res, ctx.Err()
}

func (m *Foils) copyFoil(ctx context.Context, foil catalog.FoilMirrorRow) error {
	front, err := m.convert(ctx, foil.FrontURL, storage.EncodeCard)
	if err != nil {
		return err
	}
	mask, err := m.convert(ctx, foil.MaskURL, storage.EncodeLossless)
	if err != nil {
		return err
	}
	var etch []byte
	if foil.EtchURL != "" {
		if etch, err = m.convert(ctx, foil.EtchURL, storage.EncodeLossless); err != nil {
			return err
		}
	}

	frontKey, maskKey, etchKey := FoilKeys(foil.CardID, foil.Hash)
	if err := m.store.Put(ctx, frontKey, front); err != nil {
		return err
	}
	if err := m.store.Put(ctx, maskKey, mask); err != nil {
		return err
	}
	if etch == nil {
		etchKey = ""
	} else if err := m.store.Put(ctx, etchKey, etch); err != nil {
		return err
	}
	return m.db.SetFoilKeys(ctx, foil.CardID, frontKey, maskKey, etchKey, foil.Hash)
}

func (m *Foils) convert(ctx context.Context, url string, encode func(image.Image) ([]byte, error)) ([]byte, error) {
	data, err := m.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	decoded, err := storage.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return encode(decoded.Image)
}
