package store

import (
	"context"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestAFallbackPictureOnlyFillsACardThatHasNone(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", SourceImageURL: "https://fr/125"})

	missing, err := repo.CardIDsWithoutImage(ctx, "sv03")
	if err != nil || len(missing) != 1 || !missing["sv03-001"] {
		t.Fatalf("missing = %v (%v)", missing, err)
	}
	if err := repo.SetFallbackImage(ctx, "sv03-001", "https://en/001"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetFallbackImage(ctx, "sv03-125", "https://en/125"); err != nil {
		t.Fatal(err)
	}

	var filled, kept string
	_ = tx.QueryRow(ctx, `SELECT source_image_url FROM cards WHERE id = 'sv03-001'`).Scan(&filled)
	_ = tx.QueryRow(ctx, `SELECT source_image_url FROM cards WHERE id = 'sv03-125'`).Scan(&kept)
	if filled != "https://en/001" || kept != "https://fr/125" {
		t.Errorf("filled = %q, kept = %q", filled, kept)
	}
	if again, _ := repo.CardIDsWithoutImage(ctx, "sv03"); len(again) != 0 {
		t.Errorf("nothing should be missing now: %v", again)
	}
}

func TestReimportingTheBriefListKeepsAFallbackPicture(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})
	_ = repo.SetFallbackImage(ctx, "sv03-001", "https://en/001")

	if err := repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"}); err != nil {
		t.Fatal(err)
	}

	var got string
	_ = tx.QueryRow(ctx, `SELECT source_image_url FROM cards WHERE id = 'sv03-001'`).Scan(&got)
	if got != "https://en/001" {
		t.Errorf("source image = %q", got)
	}
}

func TestASharpPictureIsStoredReplacedAndCountsAsAPicture(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})

	_ = repo.SetSharpImage(ctx, "sv03-001", "https://cdn.malie.io/v1/1.jpg")
	_ = repo.SetSharpImage(ctx, "sv03-001", "https://cdn.malie.io/v2/1.jpg")
	_ = repo.SetFallbackImage(ctx, "sv03-001", "https://en/001")

	var sharp, tcgdexStyle *string
	_ = tx.QueryRow(ctx, `SELECT sharp_image_url, source_image_url FROM cards WHERE id = 'sv03-001'`).Scan(&sharp, &tcgdexStyle)
	if sharp == nil || *sharp != "https://cdn.malie.io/v2/1.jpg" || tcgdexStyle != nil {
		t.Errorf("sharp = %v, tcgdex style = %v", sharp, tcgdexStyle)
	}
	if missing, _ := repo.CardIDsWithoutImage(ctx, "sv03"); missing["sv03-001"] {
		t.Error("a card with malie.io's picture is not without a picture")
	}
}

func TestTheMirrorSeesWhatNeedsCopyingAndWhatWasCopied(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", SourceImageURL: "https://fr/125"})
	_ = repo.SetSharpImage(ctx, "sv03-125", "https://cdn.malie.io/125.jpg")

	before, err := repo.CardsForMirroring(ctx)
	if err != nil || len(before) != 1 || before[0].ID != "sv03-125" || before[0].SharpURL == "" || before[0].SourceURL == "" || before[0].Hash != "" {
		t.Fatalf("before = %+v (%v)", before, err)
	}

	if err := repo.SetImageKeys(ctx, "sv03-125", "cards/a.webp", "cards/a.thumb.webp", "h1"); err != nil {
		t.Fatal(err)
	}

	after, _ := repo.CardsForMirroring(ctx)
	if len(after) != 1 || after[0].Hash != "h1" {
		t.Errorf("after = %+v", after)
	}
}

func TestSetsWithoutALogoCopyAreListedUntilTheirLogoIsStored(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()

	before, err := repo.SetsWithoutLogoCopy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range before {
		if s.ID == "sv03" {
			found = s.Code == "OBF"
		}
	}
	if !found {
		t.Fatalf("sv03 should be listed: %+v", before)
	}

	if err := repo.SetLogoKey(ctx, "sv03", "sets/sv03.logo.abcd1234.webp"); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.SetsWithoutLogoCopy(ctx)
	for _, s := range after {
		if s.ID == "sv03" {
			t.Error("a set whose logo was copied must not be listed again")
		}
	}
}

func TestTheImageHashOfACardIsReadAndAMissingCardIsNotAnError(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})

	if hash, exists, err := repo.CardImageHash(ctx, "sv03-001"); err != nil || !exists || hash != "" {
		t.Errorf("new card: %q, %v, %v", hash, exists, err)
	}
	_ = repo.SetImageKeys(ctx, "sv03-001", "cards/a.webp", "cards/a.thumb.webp", "h1")
	if hash, _, _ := repo.CardImageHash(ctx, "sv03-001"); hash != "h1" {
		t.Errorf("hash = %q", hash)
	}
	if _, exists, err := repo.CardImageHash(ctx, "nope-1"); err != nil || exists {
		t.Errorf("missing card: %v, %v", exists, err)
	}
}

func TestCardsWithoutAnyPictureAreListedBySet(t *testing.T) {
	repo, _ := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "A"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-002", SetID: "sv03", LocalID: "002", Name: "B", SourceImageURL: "https://assets.tcgdex.net/fr/sv/sv03/002"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-003", SetID: "sv03", LocalID: "003", Name: "C"})
	if err := repo.SetSharpImage(ctx, "sv03-003", "https://cdn.malie.io/c.png"); err != nil {
		t.Fatal(err)
	}

	got, err := repo.CardsWithoutPicture(ctx, "sv03")

	if err != nil || len(got) != 1 || got[0].CardID != "sv03-001" || got[0].LocalID != "001" {
		t.Errorf("cards = %+v (%v)", got, err)
	}
}
