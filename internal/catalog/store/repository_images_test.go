package store

import (
	"context"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// neocollects-images may give a picture-less card another language's picture: the catalog import must keep it.
func TestReimportingTheBriefListKeepsAFallbackPicture(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})
	if _, err := tx.Exec(ctx, `UPDATE cards SET source_image_url = 'https://en/001' WHERE id = 'sv03-001'`); err != nil {
		t.Fatal(err)
	}

	if err := repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"}); err != nil {
		t.Fatal(err)
	}

	var got string
	_ = tx.QueryRow(ctx, `SELECT source_image_url FROM cards WHERE id = 'sv03-001'`).Scan(&got)
	if got != "https://en/001" {
		t.Errorf("source image = %q", got)
	}
}

// neocollects-images stores a sharp picture: the catalog import must keep it too.
func TestReimportingKeepsASharpPictureAndItsCopy(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})
	if _, err := tx.Exec(ctx, `UPDATE cards SET sharp_image_url = 'https://cdn.malie.io/1.jpg' WHERE id = 'sv03-001'`); err != nil {
		t.Fatal(err)
	}
	setImageKeys(t, tx, "sv03-001", "cards/a.webp", "cards/a.thumb.webp", "h1")

	if err := repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe", SourceImageURL: "https://fr/001"}); err != nil {
		t.Fatal(err)
	}

	var sharp, key string
	_ = tx.QueryRow(ctx, `SELECT sharp_image_url, image_key FROM cards WHERE id = 'sv03-001'`).Scan(&sharp, &key)
	if sharp != "https://cdn.malie.io/1.jpg" || key != "cards/a.webp" {
		t.Errorf("sharp = %q, image key = %q", sharp, key)
	}
}
