package store

import (
	"context"
	"testing"
)

func TestAFoilIsReadBackWithItsLookAndNoKeysUntilCopied(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `INSERT INTO card_foils (card_id, look, front_source_url, mask_source_url, source_hash)
		VALUES ('30th-024', 'pillar', 'https://cdn.malie.io/f.png', 'https://cdn.malie.io/m.png', 'h')`); err != nil {
		t.Fatal(err)
	}

	row, err := q.Foil(ctx, "30th-024")
	if err != nil {
		t.Fatal(err)
	}
	if row.Look != "pillar" || row.FrontKey != nil || row.MaskKey != nil || row.EtchKey != nil {
		t.Errorf("row = %+v", row)
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
