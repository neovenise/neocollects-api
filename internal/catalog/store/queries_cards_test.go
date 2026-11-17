package store

import (
	"context"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestCardDetailWithAndWithoutPrice(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	priced, err := q.Card(ctx, "30th-024")
	if err != nil {
		t.Fatal(err)
	}
	if priced.PriceCents == nil || *priced.PriceCents != 56 || *priced.PriceCurrency != "EUR" || priced.PriceUpdatedAt == nil {
		t.Errorf("price = %v %v %v", priced.PriceCents, priced.PriceCurrency, priced.PriceUpdatedAt)
	}
	if priced.Set.ID != "30th" || priced.Set.BlockName != "Méga-Évolution" || priced.Set.OfficialCount != 128 {
		t.Errorf("set = %+v", priced.Set)
	}

	free, err := q.Card(ctx, "sv03-125")
	if err != nil {
		t.Fatal(err)
	}
	if free.PriceCents != nil || free.PriceCurrency != nil {
		t.Error("a card without price must say so")
	}
}

func TestFoilAndPriceHistory(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	foil, err := q.Foil(ctx, "sv03-125")
	if err != nil || foil.Look != "ex" || foil.FrontKey == nil || *foil.FrontKey != "f/front.webp" || foil.EtchKey != nil {
		t.Errorf("foil = %+v (%v)", foil, err)
	}

	all, _ := q.PriceHistory(ctx, "30th-024", nil, nil)
	if len(all) != 3 || all[0].Day != "2026-11-01" || all[1].Cents != 60 || all[2].Cents != 56 {
		t.Errorf("history = %+v", all)
	}
	from, to := "2026-11-05", "2026-11-12"
	ranged, _ := q.PriceHistory(ctx, "30th-024", &from, &to)
	if len(ranged) != 1 || ranged[0].Day != "2026-11-10" {
		t.Errorf("ranged history = %+v", ranged)
	}
	none, err := q.PriceHistory(ctx, "sv03-125", nil, nil)
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("a card without history: %v (%v)", none, err)
	}
}

func TestAPriceSharedWithALowerNumberedCardOfTheSameSetIsNotShown(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	repo := NewRepository(tx)
	ctx := context.Background()
	product := 918912
	day := time.Date(2026, 11, 14, 22, 0, 0, 0, time.UTC)

	for _, id := range []string{"sv03-010", "sv03-125"} {
		if err := repo.UpsertPrice(ctx, catalog.Price{CardID: id, Cents: 400, Currency: "EUR", UpdatedAt: day, ProductID: &product}); err != nil {
			t.Fatal(err)
		}
	}

	plain, err := q.Card(ctx, "sv03-010")
	if err != nil || plain.PriceCents == nil || *plain.PriceCents != 400 {
		t.Fatalf("the lowest numbered card keeps the price: %+v (%v)", plain.PriceCents, err)
	}
	alternate, err := q.Card(ctx, "sv03-125")
	if err != nil || alternate.PriceCents != nil || alternate.PriceCurrency != nil || alternate.PriceAvg1Cents != nil {
		t.Fatalf("the other one must show no price: %+v (%v)", alternate.PriceCents, err)
	}
	if points, _ := q.PriceHistory(ctx, "sv03-125", nil, nil); len(points) != 0 {
		t.Errorf("a price that is not its own has no history: %v", points)
	}
	if points, _ := q.PriceHistory(ctx, "sv03-010", nil, nil); len(points) != 1 {
		t.Errorf("the plain card keeps its history: %v", points)
	}
}

func TestTwoCardsWithTheirOwnProductsKeepTheirOwnPrices(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	repo := NewRepository(tx)
	ctx := context.Background()
	one, two := 111, 222
	day := time.Date(2026, 11, 14, 22, 0, 0, 0, time.UTC)
	_ = repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-010", Cents: 10, Currency: "EUR", UpdatedAt: day, ProductID: &one})
	_ = repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-125", Cents: 25000, Currency: "EUR", UpdatedAt: day, ProductID: &two})

	card, _ := q.Card(ctx, "sv03-125")

	if card.PriceCents == nil || *card.PriceCents != 25000 {
		t.Errorf("a price tied to its own product is shown: %v", card.PriceCents)
	}

	_ = repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-125", Cents: 25000, Currency: "EUR", UpdatedAt: day})
	if card, _ = q.Card(ctx, "sv03-125"); card.PriceCents == nil {
		t.Error("a price without product id must still be shown")
	}
}

func TestTheSameCardPrintedTwiceKeepsItsSharedPrice(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	repo := NewRepository(tx)
	ctx := context.Background()
	product := 274879
	day := time.Date(2026, 11, 14, 22, 0, 0, 0, time.UTC)

	for _, id := range []string{"sv03-901", "sv03-902"} {
		_ = repo.UpsertCard(ctx, catalog.Card{ID: id, SetID: "sv03", LocalID: id[len(id)-3:], Name: "Tortank", Rarity: "Rare"})
		if err := repo.UpsertPrice(ctx, catalog.Price{CardID: id, Cents: 900, Currency: "EUR", UpdatedAt: day, ProductID: &product}); err != nil {
			t.Fatal(err)
		}
	}

	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-903", SetID: "sv03", LocalID: "903", Name: "Tortank", Rarity: "Holo Rare"})
	_ = repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-903", Cents: 900, Currency: "EUR", UpdatedAt: day, ProductID: &product})

	for _, id := range []string{"sv03-901", "sv03-902"} {
		if card, _ := q.Card(ctx, id); card.PriceCents == nil {
			t.Errorf("%s: the shared price of the same card must be shown", id)
		}
	}
	if card, _ := q.Card(ctx, "sv03-903"); card.PriceCents != nil {
		t.Error("sv03-903: another rarity on the same product must not show the price")
	}
}

func TestTheCardDetailCarriesItsIllustrator(t *testing.T) {
	repo, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `UPDATE cards SET illustrator = 'Satoshi Ito' WHERE id = '30th-024'`); err != nil {
		t.Fatal(err)
	}
	_ = repo

	got, err := q.Card(ctx, "30th-024")

	if err != nil || got.Illustrator != "Satoshi Ito" {
		t.Errorf("illustrator = %q (%v)", got.Illustrator, err)
	}
}
