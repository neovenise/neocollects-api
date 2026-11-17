package importer

import (
	"context"
	"log/slog"
	"sync"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/catalog/tcglive"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

type PictureProber interface {
	Exists(ctx context.Context, rawURL string) (bool, error)
}

type SwshPictureStore interface {
	CardsWithoutPicture(ctx context.Context, setID string) ([]catalog.FoilCandidate, error)
}

type SwshPictures struct {
	prober PictureProber
	store  SwshPictureStore
	logger *slog.Logger
}

func NewSwshPictures(prober PictureProber, store SwshPictureStore, logger *slog.Logger) *SwshPictures {
	return &SwshPictures{prober: prober, store: store, logger: logger}
}

const picturesAtOnce = 4

// SharpImages returns game pictures for cards that have none.
func (p *SwshPictures) SharpImages(ctx context.Context, setID string) (map[string]string, error) {
	set, ok := tcglive.SwshSets[setID]
	if !ok {
		return nil, nil
	}
	cards, err := p.store.CardsWithoutPicture(ctx, setID)
	if err != nil {
		return nil, err
	}
	var (
		mu     sync.Mutex
		found  = map[string]string{}
		failed int
		wg     sync.WaitGroup
		work   = make(chan catalog.FoilCandidate)
	)
	for w := 0; w < picturesAtOnce; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for card := range work {
				front, ok, err := p.faceOf(ctx, set, card)
				mu.Lock()
				switch {
				case err != nil:
					failed++
				case ok:
					found[card.CardID] = front
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for _, card := range cards {
		select {
		case work <- card:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	if failed > 0 {
		p.logger.Warn("game pictures: some requests failed, those cards will be asked about again", "set", setID, "failed", failed)
	}
	return found, ctx.Err()
}

func (p *SwshPictures) faceOf(ctx context.Context, set tcglive.GameSet, card catalog.FoilCandidate) (string, bool, error) {
	number, ok := tcglive.GameNumber(card.LocalID, set.Offset)
	if !ok {
		return "", false, nil
	}
	for _, lang := range tcglive.Languages {
		front, _, _ := malie.PictureURLs(lang, set.Code, number)
		exists, err := p.prober.Exists(ctx, front)
		if err != nil {
			return "", false, err
		}
		if exists {
			return front, true, nil
		}
	}
	return "", false, nil
}

// SharpImageSources merges several sources; the first one wins.
func SharpImageSources(sources ...SharpImageSource) SharpImageSource { return sharpList(sources) }

type sharpList []SharpImageSource

func (l sharpList) SharpImages(ctx context.Context, setID string) (map[string]string, error) {
	all := map[string]string{}
	var firstErr error
	for _, source := range l {
		found, err := source.SharpImages(ctx, setID)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		for id, url := range found {
			if _, taken := all[id]; !taken {
				all[id] = url
			}
		}
	}
	if len(all) > 0 {
		return all, nil
	}
	return all, firstErr
}
