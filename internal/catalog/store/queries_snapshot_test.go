package store

import (
	"context"
	"testing"
)

func TestSnapshotQueryIsPhysicalOnlyAndKeepsPromos(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	sets, cards, err := q.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for _, c := range cards {
		ids[c.ID] = true
	}
	if ids["tcgp-a1-024"] {
		t.Error("the digital game must not be in the snapshot")
	}
	if !ids["30th-TG01"] || !ids["sv03-125"] {
		t.Errorf("cards = %v", ids)
	}
	for _, s := range sets {
		if s.ID == "tcgp-a1" {
			t.Error("a digital set must not be in the snapshot")
		}
	}
	if len(sets) != 4 || len(cards) != 7 {
		t.Errorf("%d sets, %d cards", len(sets), len(cards))
	}
}
