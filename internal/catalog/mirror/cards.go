package mirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

const mirrorVersion = "1"

type CardStore interface {
	CardsForMirroring(ctx context.Context) ([]catalog.MirrorCard, error)
	SetImageKeys(ctx context.Context, cardID, imageKey, thumbKey, hash string) error
}

type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
}

type Result struct {
	Copied int

	UpToDate int
	Failed   []string
}

type Cards struct {
	db      CardStore
	fetcher Fetcher
	store   Storage
	logger  *slog.Logger
}

func NewCards(db CardStore, fetcher Fetcher, store Storage, logger *slog.Logger) *Cards {
	return &Cards{db: db, fetcher: fetcher, store: store, logger: logger}
}

func sourceOf(c catalog.MirrorCard) (url string, ok bool) {
	switch {
	case c.SharpURL != "":
		return c.SharpURL, true
	case c.SourceURL != "":
		return c.SourceURL + "/high.webp", true
	}
	return "", false
}

// SourceHash identifies a source picture at this mirror version.
func SourceHash(sourceURL string) string {
	sum := sha256.Sum256([]byte(mirrorVersion + "|" + sourceURL))
	return hex.EncodeToString(sum[:16])
}

// ObjectKeys returns the keys of a card's picture and thumbnail.
func ObjectKeys(cardID, hash string) (image, thumb string) {
	safe := safeKey(cardID)
	short := hash[:8]
	return fmt.Sprintf("cards/%s.%s.webp", safe, short), fmt.Sprintf("cards/%s.%s.thumb.webp", safe, short)
}

// Run copies the card pictures that are missing or stale.
func (m *Cards) Run(ctx context.Context, concurrency int) (Result, error) {
	var res Result
	if concurrency <= 0 {
		concurrency = 4
	}
	cards, err := m.db.CardsForMirroring(ctx)
	if err != nil {
		return res, err
	}
	var todo []catalog.MirrorCard
	for _, c := range cards {
		source, ok := sourceOf(c)
		if !ok {
			continue
		}
		if c.Hash == SourceHash(source) {
			res.UpToDate++
			continue
		}
		todo = append(todo, c)
	}

	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		slots = make(chan struct{}, concurrency)
	)
	for _, card := range todo {
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
			err := m.copyCard(ctx, card)

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				res.Copied++
				if res.Copied%500 == 0 {
					m.logger.Info("pictures copied", "done", res.Copied, "to_do", len(todo))
				}
			case errors.Is(err, context.Canceled):
			default:
				m.logger.Warn("could not copy a picture", "card", card.ID, "error", err)
				res.Failed = append(res.Failed, card.ID)
			}
		}()
	}
	wg.Wait()
	return res, ctx.Err()
}

func (m *Cards) copyCard(ctx context.Context, card catalog.MirrorCard) error {
	source, _ := sourceOf(card)
	data, err := m.fetcher.Fetch(ctx, source)
	if err != nil {

		other, ok := englishTCGdexURL(source)
		if !ok {
			return err
		}
		var errEnglish error
		if data, errEnglish = m.fetcher.Fetch(ctx, other); errEnglish != nil {
			return err
		}
		source = other
	}
	decoded, err := storage.Decode(data)
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}

	cardFile := data
	if decoded.Format != "webp" {
		if cardFile, err = storage.EncodeCard(decoded.Image); err != nil {
			return err
		}
	}
	thumbFile, err := storage.EncodeThumb(decoded.Image)
	if err != nil {
		return err
	}

	hash := SourceHash(source)
	imageKey, thumbKey := ObjectKeys(card.ID, hash)
	if err := m.store.Put(ctx, imageKey, cardFile); err != nil {
		return err
	}
	if err := m.store.Put(ctx, thumbKey, thumbFile); err != nil {
		return err
	}
	return m.db.SetImageKeys(ctx, card.ID, imageKey, thumbKey, hash)
}

func safeKey(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.', r == '_':
			return r
		}
		return '_'
	}, id)
}

func englishTCGdexURL(url string) (string, bool) {
	const host = "https://assets.tcgdex.net/fr/"
	if !strings.HasPrefix(url, host) {
		return "", false
	}
	return "https://assets.tcgdex.net/en/" + strings.TrimPrefix(url, host), true
}
