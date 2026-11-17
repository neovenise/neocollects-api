package store

import (
	"context"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestFacetsCountTheValuesOfPhysicalCardsAndLeaveOutEmptyOnes(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	f, err := q.Facets(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	counts := func(values []catalog.FacetValue) map[string]int {
		m := map[string]int{}
		for _, v := range values {
			m[v.Value] = v.Count
		}
		return m
	}
	if r := counts(f.Rarities); r["Ultra Rare"] != 2 || r["Rare"] != 2 || r["Diamant"] != 0 || r[""] != 0 {
		t.Errorf("rarities = %v (the digital Diamant must not appear)", f.Rarities)
	}
	if c := counts(f.Categories); c["Pokémon"] != 4 || c["Dresseur"] != 1 || len(c) != 2 {
		t.Errorf("categories = %v (the cards without detail have none)", f.Categories)
	}
	if ty := counts(f.Types); ty["Plante"] != 1 || ty["Psy"] != 1 || ty["Feu"] != 0 || len(ty) != 5 {
		t.Errorf("types = %v (a dual-type card counts once in each)", f.Types)
	}
	if len(f.Stages) != 3 || len(f.TrainerTypes) != 1 || f.TrainerTypes[0].Value != "Objet" || len(f.RegulationMarks) != 2 {
		t.Errorf("stages/trainer types/marks = %v / %v / %v", f.Stages, f.TrainerTypes, f.RegulationMarks)
	}
	if f.RegulationMarks[0].Value != "I" || f.RegulationMarks[0].Count != 3 {
		t.Errorf("marks = %v, want the most common first (I: 3)", f.RegulationMarks)
	}
}
