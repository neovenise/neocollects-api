package store

import (
	"context"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestAnEmptyRarityNeverErasesAKnownOne(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Rarity: "Double rare", SourceImageURL: "https://img/125"})

	if err := repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"}); err != nil {
		t.Fatal(err)
	}

	var rarity, image string
	if err := tx.QueryRow(ctx, `SELECT rarity, source_image_url FROM cards WHERE id = 'sv03-125'`).Scan(&rarity, &image); err != nil {
		t.Fatal(err)
	}
	if rarity != "Double rare" || image != "https://img/125" {
		t.Errorf("rarity = %q, image = %q", rarity, image)
	}
}

func TestCardSearchColumns(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-024", SetID: "sv03", LocalID: "024", Name: "Ékaïser-ex"})
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-TG01", SetID: "sv03", LocalID: "TG01", Name: "Pikachu"})

	var number *int
	var search string
	if err := tx.QueryRow(ctx, `SELECT local_number, name_search FROM cards WHERE id = 'sv03-024'`).Scan(&number, &search); err != nil {
		t.Fatal(err)
	}
	if number == nil || *number != 24 || search != "ekaiser ex" {
		t.Errorf("number = %v, search = %q", number, search)
	}
	if err := tx.QueryRow(ctx, `SELECT local_number FROM cards WHERE id = 'sv03-TG01'`).Scan(&number); err != nil || number != nil {
		t.Errorf("a lettered number must stay NULL: %v (%v)", number, err)
	}
}

func TestSetUpsertIsIdempotentAndKeepsTheReleaseDate(t *testing.T) {
	repo, tx := newTx(t)
	ctx := context.Background()
	_ = repo.UpsertBlock(ctx, catalog.Block{ID: "me", Name: "Méga-Évolution", Position: 1})
	date := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	set := catalog.Set{ID: "30th", Code: "30C", Name: "30ᵉ Anniversaire", BlockID: "me", OfficialCount: 128, TotalCount: 161, ReleaseDate: &date}

	for range 2 {
		if err := repo.UpsertSet(ctx, set); err != nil {
			t.Fatal(err)
		}
	}

	var n int
	var release string
	if err := tx.QueryRow(ctx, `SELECT count(*), max(release_date::text) FROM sets WHERE id = '30th'`).Scan(&n, &release); err != nil {
		t.Fatal(err)
	}
	if n != 1 || release != "2026-09-16" {
		t.Errorf("rows = %d, release = %q", n, release)
	}
}

func TestCardIDsAndSetCount(t *testing.T) {
	repo, _ := newTx(t)
	ctx := context.Background()
	seed(t, repo)
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe"})

	ids, err := repo.CardIDs(ctx, "sv03")
	if err != nil || len(ids) != 1 || !ids["sv03-001"] {
		t.Errorf("CardIDs = %v (%v)", ids, err)
	}
	if n, err := repo.SetCount(ctx); err != nil || n < 1 {
		t.Errorf("SetCount = %d (%v)", n, err)
	}
}

func TestEnsureBlockKeepsAnExistingPosition(t *testing.T) {
	repo, tx := newTx(t)
	ctx := context.Background()
	_ = repo.UpsertBlock(ctx, catalog.Block{ID: "sv", Name: "Écarlate et Violet", Position: 2})

	if err := repo.EnsureBlock(ctx, catalog.Block{ID: "sv", Name: "other", Position: 99}); err != nil {
		t.Fatal(err)
	}

	var name string
	var position int
	_ = tx.QueryRow(ctx, `SELECT name, position FROM blocks WHERE id = 'sv'`).Scan(&name, &position)
	if name != "Écarlate et Violet" || position != 2 {
		t.Errorf("block = %q at %d", name, position)
	}
}

func TestReimportingKeepsTheDigitalFlag(t *testing.T) {
	repo, tx := newTx(t)
	ctx := context.Background()

	_ = repo.UpsertBlock(ctx, catalog.Block{ID: "tcgp", Name: "Pocket", Position: 4, Digital: catalog.IsDigitalBlock("tcgp")})
	_ = repo.UpsertBlock(ctx, catalog.Block{ID: "tcgp", Name: "Pocket", Position: 5, Digital: catalog.IsDigitalBlock("tcgp")})

	var digital bool
	if err := tx.QueryRow(ctx, `SELECT is_digital FROM blocks WHERE id = 'tcgp'`).Scan(&digital); err != nil || !digital {
		t.Errorf("is_digital = %v (%v)", digital, err)
	}
}

func TestReimportingTheBriefListDoesNotEraseWhatTheDetailGave(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	repo := NewRepository(tx)

	if err := repo.UpsertCard(context.Background(), catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"}); err != nil {
		t.Fatal(err)
	}

	if got := search(t, q, catalog.CardFilter{Types: []string{"Obscurité"}, Stages: []string{"Niveau 2"}, RegulationMarks: []string{"G"}, Categories: []string{"Pokémon"}}); !sameSet(got, "sv03-125") {
		t.Errorf("found %v: the attributes were erased", got)
	}

	if err := repo.UpsertCard(context.Background(), catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Types: []string{"Feu"}}); err != nil {
		t.Fatal(err)
	}
	if got := search(t, q, catalog.CardFilter{Types: []string{"Feu"}}); !sameSet(got, "sv03-125") {
		t.Errorf("found %v: a new type was not stored", got)
	}
	if got := search(t, q, catalog.CardFilter{Types: []string{"Obscurité"}}); len(got) != 0 {
		t.Errorf("found %v: the old type is still there", got)
	}
}

func TestAnEmptyIllustratorNeverErasesAKnownOne(t *testing.T) {
	repo, tx := newTx(t)
	seed(t, repo)
	ctx := context.Background()
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Illustrator: "5ban Graphics"})

	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"})

	var got string
	if err := tx.QueryRow(ctx, `SELECT illustrator FROM cards WHERE id = 'sv03-125'`).Scan(&got); err != nil || got != "5ban Graphics" {
		t.Errorf("illustrator = %q (%v)", got, err)
	}
	_ = repo.UpsertCard(ctx, catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Illustrator: "Mitsuhiro Arita"})
	_ = tx.QueryRow(ctx, `SELECT illustrator FROM cards WHERE id = 'sv03-125'`).Scan(&got)
	if got != "Mitsuhiro Arita" {
		t.Errorf("illustrator = %q", got)
	}
}
