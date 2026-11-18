package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/catalog"
)

func intPtr(n int) *int { return &n }

func date(s string) *time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return &t
}

func seedCatalog(t *testing.T, tx pgx.Tx) *Queries {
	t.Helper()
	ctx := context.Background()
	repo := NewRepository(tx)

	for _, b := range []catalog.Block{
		{ID: "me", Name: "Méga-Évolution", Position: 1},
		{ID: "sv", Name: "Écarlate et Violet", Position: 2},
		{ID: "swsh", Name: "Épée et Bouclier", Position: 3},
		{ID: "tcgp", Name: "Pocket", Position: 4, Digital: true},
	} {
		if err := repo.UpsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []catalog.Set{
		{ID: "30th", Code: "30C", Name: "30ᵉ Anniversaire", BlockID: "me", OfficialCount: 128, TotalCount: 161, ReleaseDate: date("2026-09-16")},
		{ID: "sv03", Code: "OBF", Name: "Flammes Obsidiennes", BlockID: "sv", OfficialCount: 197, TotalCount: 230, ReleaseDate: date("2023-08-11")},
		{ID: "swsh-a", Name: "Épée A", BlockID: "swsh", OfficialCount: 30, TotalCount: 40, ReleaseDate: date("2021-01-01")},
		{ID: "swsh-b", Name: "Épée B", BlockID: "swsh", OfficialCount: 30, TotalCount: 40, ReleaseDate: date("2022-01-01")},
		{ID: "tcgp-a1", Name: "Pocket A1", BlockID: "tcgp", OfficialCount: 30, TotalCount: 30, ReleaseDate: date("2024-10-30")},
	} {
		if err := repo.UpsertSet(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []catalog.Card{
		{ID: "30th-024", SetID: "30th", LocalID: "024", Name: "Pikachu", Rarity: "Pikachu Rare",
			Category: "Pokémon", Types: []string{"Électrique"}, HP: intPtr(60), Stage: "Base", RegulationMark: "I"},
		{ID: "30th-001", SetID: "30th", LocalID: "001", Name: "Noeunoeuf", Rarity: "Commune",
			Category: "Pokémon", Types: []string{"Plante", "Psy"}, HP: intPtr(70), Stage: "Base", RegulationMark: "I"},
		{ID: "30th-TG01", SetID: "30th", LocalID: "TG01", Name: "Galerie", Rarity: "Rare", Category: "Dresseur", TrainerType: "Objet", RegulationMark: "I"},
		{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Rarity: "Double rare",
			Category: "Pokémon", Types: []string{"Obscurité"}, HP: intPtr(330), Stage: "Niveau 2", RegulationMark: "G"},
		{ID: "sv03-010", SetID: "sv03", LocalID: "010", Name: "Ékaïser", Rarity: "Rare",
			Category: "Pokémon", Types: []string{"Métal"}, HP: intPtr(120), Stage: "Niveau 1", RegulationMark: "G"},
		{ID: "swsh-a-024", SetID: "swsh-a", LocalID: "024", Name: "Pikachu V", Rarity: "Ultra Rare"},
		{ID: "swsh-b-024", SetID: "swsh-b", LocalID: "024", Name: "Pikachu VMAX", Rarity: "Ultra Rare"},
		{ID: "tcgp-a1-024", SetID: "tcgp-a1", LocalID: "024", Name: "Pikachu Pocket", Rarity: "Diamant"},
	} {
		if err := repo.UpsertCard(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	setImageKeys(t, tx, "30th-024", "cards/30th-024.webp", "cards/30th-024.thumb.webp", "h")
	for _, p := range []catalog.Price{price("30th-024", 56, "2026-11-01"), price("30th-024", 60, "2026-11-10"), price("30th-024", 56, "2026-11-14")} {
		if err := repo.UpsertPrice(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO card_foils (card_id, look, front_key, mask_key, etch_key, source_hash)
		VALUES ('sv03-125', 'ex', 'f/front.webp', 'f/mask.png', NULL, 'h')`); err != nil {
		t.Fatal(err)
	}
	return NewQueries(tx)
}

func ids(cards []catalog.CardRow) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func search(t *testing.T, q *Queries, f catalog.CardFilter) []string {
	t.Helper()
	f.Limit = 50
	cards, _, err := q.SearchCards(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	return ids(cards)
}

func sameSet(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			return false
		}
	}
	return true
}
