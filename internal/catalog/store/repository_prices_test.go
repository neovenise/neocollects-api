package store

import (
	"context"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestPriceHistoryOnlyGrowsWhenThePriceChanges(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	if err := repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"}); err != nil {
		t.Fatal(err)
	}

	steps := []catalog.Price{
		price("sv03-125", 100, "2026-11-01"),
		price("sv03-125", 100, "2026-11-02"),
		price("sv03-125", 100, "2026-11-02"),
		price("sv03-125", 150, "2026-11-03"),
		price("sv03-125", 100, "2026-11-04"),
		price("sv03-125", 120, "2026-11-04"),
	}
	for _, p := range steps {
		if err := repo.UpsertPrice(ctx, p); err != nil {
			t.Fatalf("UpsertPrice(%+v): %v", p, err)
		}
	}

	want := []historyRow{{"2026-11-01", 100}, {"2026-11-03", 150}, {"2026-11-04", 120}}
	got := history(t, tx, "sv03-125")
	if len(got) != len(want) {
		t.Fatalf("history = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("history[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	var cents int
	if err := tx.QueryRow(ctx, `SELECT cents FROM card_prices WHERE card_id = 'sv03-125'`).Scan(&cents); err != nil || cents != 120 {
		t.Errorf("current price = %d (%v), want 120", cents, err)
	}
}

func TestPriceDayIsTheSourceDayWhateverTheSessionTimeZone(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"})
	if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'Pacific/Auckland'`); err != nil {
		t.Fatal(err)
	}

	if err := repo.UpsertPrice(ctx, price("sv03-125", 100, "2026-11-14")); err != nil {
		t.Fatal(err)
	}

	if got := history(t, tx, "sv03-125"); len(got) != 1 || got[0].Day != "2026-11-14" {
		t.Errorf("history = %v, want the day 2026-11-14", got)
	}
}

func TestPriceAveragesAreStoredAndRefreshedEvenWhenTheTrendDoesNotChange(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"})
	day, week, month := 365, 376, 378
	updated := time.Date(2026, 11, 14, 22, 0, 0, 0, time.UTC)

	if err := repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-125", Cents: 402, Currency: "EUR", UpdatedAt: updated, Avg1Cents: &day, Avg7Cents: &week, Avg30Cents: &month}); err != nil {
		t.Fatal(err)
	}
	newer := 370
	if err := repo.UpsertPrice(ctx, catalog.Price{CardID: "sv03-125", Cents: 402, Currency: "EUR", UpdatedAt: updated.Add(24 * time.Hour), Avg1Cents: &newer}); err != nil {
		t.Fatal(err)
	}

	var a1, a7, a30 *int
	_ = tx.QueryRow(ctx, `SELECT avg1_cents, avg7_cents, avg30_cents FROM card_prices WHERE card_id = 'sv03-125'`).Scan(&a1, &a7, &a30)
	if a1 == nil || *a1 != 370 || a7 != nil || a30 != nil {
		t.Errorf("averages = %v %v %v, want 370 nil nil (what the source gave last)", a1, a7, a30)
	}
}
