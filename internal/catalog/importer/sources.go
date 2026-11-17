package importer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/catalog/tcglive"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

type ImageFallback interface {
	Images(ctx context.Context, setID string, missing map[string]bool) (map[string]string, error)
}

type SharpImageSource interface {
	SharpImages(ctx context.Context, setID string) (map[string]string, error)
}

type CardSupplement interface {
	MissingCards(ctx context.Context, setID string) ([]catalog.Card, error)
}

type EnglishImages struct {
	source Source
}

func NewEnglishImages(source Source) *EnglishImages { return &EnglishImages{source: source} }

// Images returns English pictures for cards without a French one.
func (e *EnglishImages) Images(ctx context.Context, setID string, missing map[string]bool) (map[string]string, error) {
	set, err := e.source.Set(ctx, setID)
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for _, card := range set.Cards {
		if card.Image != "" && missing[card.ID] {
			found[card.ID] = card.Image
		}
	}
	return found, nil
}

type MalieImageStore interface {
	SetCode(ctx context.Context, setID string) (string, error)

	SetsWithCode(ctx context.Context, code string) (int, error)
	CardIDsByNumber(ctx context.Context, setID string) (map[int]string, error)
}

type extraCard struct {
	CardID   string
	MalieKey string
	Number   int
}

var malieExtraCards = map[string][]extraCard{
	"30th": {
		{CardID: "30th-R", MalieKey: "me5-5m", Number: 1},
		{CardID: "30th-G", MalieKey: "me5-5m", Number: 2},
		{CardID: "30th-B", MalieKey: "me5-5m", Number: 3},
	},
}

type MalieImages struct {
	source malie.Source
	store  MalieImageStore

	mu    sync.Mutex
	index map[string]malie.IndexEntry
	byKey map[string]malie.IndexEntry

	files map[string]cachedFile
	now   func() time.Time
}

type cachedFile struct {
	cards []malie.Card
	at    time.Time
}

const fileCacheTTL = 10 * time.Minute

func NewMalieImages(source malie.Source, store MalieImageStore) *MalieImages {
	return &MalieImages{source: source, store: store, files: map[string]cachedFile{}, now: time.Now}
}

func (m *MalieImages) entry(ctx context.Context, code string) (malie.IndexEntry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == nil {
		entries, err := m.source.Index(ctx)
		if err != nil {
			return malie.IndexEntry{}, false, err
		}
		m.index = make(map[string]malie.IndexEntry, len(entries))
		m.byKey = entries
		for _, e := range entries {
			if abbr := strings.ToUpper(strings.TrimSpace(e.Abbr)); abbr != "" {
				m.index[abbr] = e
			}
		}
	}
	if alias, ok := tcglive.ExportCodes[code]; ok {
		code = alias
	}
	e, ok := m.index[code]
	return e, ok, nil
}

func (m *MalieImages) entryByKey(ctx context.Context, key string) (malie.IndexEntry, bool, error) {
	if _, _, err := m.entry(ctx, ""); err != nil {
		return malie.IndexEntry{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byKey[key]
	return e, ok, nil
}

// SharpImages returns the high-resolution picture of a set's cards.
func (m *MalieImages) SharpImages(ctx context.Context, setID string) (map[string]string, error) {
	cards, ok, err := m.setCards(ctx, setID)
	if err != nil || !ok {
		return nil, err
	}
	ids, err := m.store.CardIDsByNumber(ctx, setID)
	if err != nil {
		return nil, err
	}
	found := map[string]string{}
	for _, card := range cards {
		if card.IsReverseVariant() || card.CollectorNumber.Numeric == nil {
			continue
		}
		url, ok := card.PreviewImage()
		if !ok {
			continue
		}
		if id, known := ids[*card.CollectorNumber.Numeric]; known {
			found[id] = url
		}
	}
	if err := m.addExtraCards(ctx, setID, found); err != nil {
		return nil, err
	}
	return found, nil
}

func (m *MalieImages) addExtraCards(ctx context.Context, setID string, found map[string]string) error {
	for _, extra := range malieExtraCards[setID] {
		entry, ok, err := m.entryByKey(ctx, extra.MalieKey)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		cards, err := m.fileCards(ctx, entry)
		if err != nil {
			return fmt.Errorf("malie cards of %s: %w", extra.MalieKey, err)
		}
		for _, card := range cards {
			if card.IsReverseVariant() || card.CollectorNumber.Numeric == nil || *card.CollectorNumber.Numeric != extra.Number {
				continue
			}
			if url, ok := card.PreviewImage(); ok {
				found[extra.CardID] = url
			}
			break
		}
	}
	return nil
}

func (m *MalieImages) setCards(ctx context.Context, setID string) (cards []malie.Card, ok bool, err error) {
	var entry malie.IndexEntry
	if key, linked := tcglive.ExportSets[setID]; linked {
		entry, ok, err = m.entryByKey(ctx, key)
	} else {
		entry, ok, err = m.entryByCode(ctx, setID)
	}
	if err != nil || !ok {
		return nil, false, err
	}
	cards, err = m.fileCards(ctx, entry)
	if err != nil {
		return nil, false, fmt.Errorf("malie cards of %s: %w", setID, err)
	}
	return cards, true, nil
}

func (m *MalieImages) fileCards(ctx context.Context, entry malie.IndexEntry) ([]malie.Card, error) {
	m.mu.Lock()
	cached, hit := m.files[entry.Path]
	m.mu.Unlock()
	if hit && m.now().Sub(cached.at) < fileCacheTTL {
		return cached.cards, nil
	}
	cards, err := m.source.Set(ctx, entry.Path)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.files[entry.Path] = cachedFile{cards: cards, at: m.now()}
	m.mu.Unlock()
	return cards, nil
}

// MissingCards returns the set's cards missing from the catalog.
func (m *MalieImages) MissingCards(ctx context.Context, setID string) ([]catalog.Card, error) {
	cards, ok, err := m.setCards(ctx, setID)
	if err != nil || !ok {
		return nil, err
	}
	ids, err := m.store.CardIDsByNumber(ctx, setID)
	if err != nil {
		return nil, err
	}
	var missing []catalog.Card
	seen := map[int]bool{}
	for _, card := range cards {
		if card.IsReverseVariant() || card.CollectorNumber.Numeric == nil || card.Name == "" {
			continue
		}
		number := *card.CollectorNumber.Numeric
		if _, known := ids[number]; known || seen[number] || number <= 0 {
			continue
		}
		seen[number] = true
		localID := fmt.Sprintf("%03d", number)
		missing = append(missing, catalog.Card{ID: setID + "-" + localID, SetID: setID, LocalID: localID, Name: card.Name})
	}
	return missing, nil
}

func (m *MalieImages) entryByCode(ctx context.Context, setID string) (malie.IndexEntry, bool, error) {
	code, err := m.store.SetCode(ctx, setID)
	if err != nil || code == "" {
		return malie.IndexEntry{}, false, err
	}

	if n, err := m.store.SetsWithCode(ctx, code); err != nil || n != 1 {
		return malie.IndexEntry{}, false, err
	}
	return m.entry(ctx, code)
}
