package store

import (
	"context"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestFoilsAreStoredFromTheirSourceUrlsAndReadBack(t *testing.T) {
	repo, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()
	foil := catalog.Foil{CardID: "30th-024", Look: "holo", FrontURL: "https://cdn.malie.io/f.png", MaskURL: "https://cdn.malie.io/m.png", SourceHash: "h1"}

	for range 2 {
		if err := repo.UpsertFoil(ctx, foil); err != nil {
			t.Fatal(err)
		}
	}
	foil.Look, foil.EtchURL, foil.SourceHash = "pillar", "https://cdn.malie.io/e.png", "h2"
	if err := repo.UpsertFoil(ctx, foil); err != nil {
		t.Fatal(err)
	}

	row, err := q.Foil(ctx, "30th-024")
	if err != nil {
		t.Fatal(err)
	}
	if row.Look != "pillar" || row.FrontKey != nil || row.MaskKey != nil || row.EtchKey != nil {
		t.Errorf("row = %+v", row)
	}
	var n int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM card_foils WHERE card_id = '30th-024'`).Scan(&n)
	if n != 1 {
		t.Errorf("%d rows for one card", n)
	}
	if count, err := repo.FoilCount(ctx); err != nil || count < 2 {
		t.Errorf("FoilCount = %d (%v)", count, err)
	}
}

func TestAFoilNeedsPicturesFromSomewhere(t *testing.T) {
	_, tx := newTx(t)
	seedCatalog(t, tx)

	_, err := tx.Exec(context.Background(), `INSERT INTO card_foils (card_id, look, source_hash) VALUES ('30th-024', 'holo', 'h')`)

	if err == nil {
		t.Error("a foil with neither keys nor source URLs must be refused by the database")
	}
}

func TestFoilsToMirrorAreListedUntilTheirPicturesAreCopiedFromTheCurrentSource(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"})
	if err := repo.UpsertFoil(ctx, catalog.Foil{CardID: "sv03-125", Look: "ex", FrontURL: "https://cdn.malie.io/f.png", MaskURL: "https://cdn.malie.io/m.png", SourceHash: "v1"}); err != nil {
		t.Fatal(err)
	}

	todo, err := repo.FoilsToMirror(ctx)
	if err != nil || len(todo) != 1 || todo[0].CardID != "sv03-125" || todo[0].EtchURL != "" || todo[0].CopiedKey {
		t.Fatalf("to mirror = %+v (%v)", todo, err)
	}

	if err := repo.SetFoilKeys(ctx, "sv03-125", "foils/f.webp", "foils/m.webp", "", "v1"); err != nil {
		t.Fatal(err)
	}
	if todo, _ = repo.FoilsToMirror(ctx); len(todo) != 0 {
		t.Errorf("copied from the current source: nothing left to do, got %+v", todo)
	}

	_ = repo.UpsertFoil(ctx, catalog.Foil{CardID: "sv03-125", Look: "ex", FrontURL: "https://cdn.malie.io/f2.png", MaskURL: "https://cdn.malie.io/m2.png", SourceHash: "v2"})
	if todo, _ = repo.FoilsToMirror(ctx); len(todo) != 1 || !todo[0].CopiedKey {
		t.Errorf("a changed source must be copied again: %+v", todo)
	}
}

func TestOnlyTheCardsOfTheGivenSetsWithoutAFoilAreListed(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "A", Rarity: "Commune"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "B", Rarity: "Holo Rare"})
	if err := repo.UpsertFoil(ctx, catalog.Foil{CardID: "sv03-125", Look: "ex", FrontURL: "https://cdn.malie.io/f.png", MaskURL: "https://cdn.malie.io/m.png", SourceHash: "h"}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FoillessCards(ctx, []string{"sv03"})
	if err != nil || len(got) != 1 || got[0].CardID != "sv03-001" || got[0].Rarity != "Commune" || got[0].LocalID != "001" {
		t.Errorf("cards = %+v (%v)", got, err)
	}
	if other, _ := repo.FoillessCards(ctx, []string{"nothing"}); len(other) != 0 {
		t.Errorf("another set: %+v", other)
	}
}
