package store

import (
	"context"
	"errors"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestBlocksAreNewestFirstAndPhysical(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	blocks, err := q.Blocks(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, b := range blocks {
		got = append(got, b.ID)
	}
	if !equal(got, []string{"me", "sv", "swsh"}) {
		t.Errorf("blocks = %v (the digital block must be hidden)", got)
	}
}

func TestSetsOrderAndFilters(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	all, _ := q.Sets(ctx, catalog.SetFilter{})
	var order []string
	for _, s := range all {
		order = append(order, s.ID)
	}
	if !equal(order, []string{"30th", "sv03", "swsh-b", "swsh-a"}) {
		t.Errorf("order = %v, want block by block, newest set first", order)
	}

	size := 30
	bySize, _ := q.Sets(ctx, catalog.SetFilter{OfficialCount: &size})
	if len(bySize) != 2 {
		t.Errorf("size 30 = %d sets, want the 2 physical ones (Pocket hidden)", len(bySize))
	}
	byBlock, _ := q.Sets(ctx, catalog.SetFilter{BlockID: "sv"})
	if len(byBlock) != 1 || byBlock[0].BlockName != "Écarlate et Violet" || byBlock[0].Code != "OBF" {
		t.Errorf("block filter = %+v", byBlock)
	}
}

func TestDigitalAndUnknownThingsAreNotFound(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	if _, err := q.Set(ctx, "tcgp-a1"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("digital set: %v", err)
	}
	if _, err := q.SetCards(ctx, "tcgp-a1"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("digital set cards: %v", err)
	}
	if _, err := q.Card(ctx, "tcgp-a1-024"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("digital card: %v", err)
	}
	if _, err := q.Card(ctx, "nope"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("unknown card: %v", err)
	}
	if _, err := q.Foil(ctx, "30th-024"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("a card without foil: %v", err)
	}
	if _, err := q.PriceHistory(ctx, "nope", nil, nil); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("history of an unknown card: %v", err)
	}
}

func TestSetCardsAreInPrintedOrderLettersLast(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	cards, err := q.SetCards(context.Background(), "30th")
	if err != nil {
		t.Fatal(err)
	}

	if got := ids(cards); !equal(got, []string{"30th-001", "30th-024", "30th-TG01"}) {
		t.Errorf("order = %v", got)
	}
}
