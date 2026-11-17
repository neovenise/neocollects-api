package store

import (
	"context"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// UpsertFoil stores the foil of a card.
func (r *Repository) UpsertFoil(ctx context.Context, f catalog.Foil) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO card_foils (card_id, look, front_source_url, mask_source_url, etch_source_url, source_hash)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		ON CONFLICT (card_id) DO UPDATE SET
			look = EXCLUDED.look, front_source_url = EXCLUDED.front_source_url, mask_source_url = EXCLUDED.mask_source_url,
			etch_source_url = EXCLUDED.etch_source_url, source_hash = EXCLUDED.source_hash`,
		f.CardID, f.Look, f.FrontURL, f.MaskURL, f.EtchURL, f.SourceHash)
	return wrap("upsert foil", err)
}

// FoilCount returns how many foils are stored.
func (r *Repository) FoilCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM card_foils`).Scan(&n)
	return n, wrap("count foils", err)
}

// FoilsToMirror lists foils whose copied pictures are missing or stale.
func (r *Repository) FoilsToMirror(ctx context.Context) ([]catalog.FoilMirrorRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT f.card_id, COALESCE(f.front_source_url, ''), COALESCE(f.mask_source_url, ''), COALESCE(f.etch_source_url, ''),
		       f.source_hash, f.front_key IS NOT NULL
		FROM card_foils f
		JOIN cards c ON c.id = f.card_id JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital AND f.front_source_url IS NOT NULL AND f.mask_source_url IS NOT NULL
		  AND (f.front_key IS NULL OR f.mask_key IS NULL OR f.copied_hash IS DISTINCT FROM f.source_hash)
		ORDER BY f.card_id`)
	if err != nil {
		return nil, wrap("list foils to mirror", err)
	}
	defer rows.Close()

	var foils []catalog.FoilMirrorRow
	for rows.Next() {
		var f catalog.FoilMirrorRow
		if err := rows.Scan(&f.CardID, &f.FrontURL, &f.MaskURL, &f.EtchURL, &f.Hash, &f.CopiedKey); err != nil {
			return nil, wrap("scan foil", err)
		}
		foils = append(foils, f)
	}
	return foils, wrap("list foils to mirror", rows.Err())
}

// SetFoilKeys records where a foil's copied pictures are stored.
func (r *Repository) SetFoilKeys(ctx context.Context, cardID, frontKey, maskKey, etchKey, hash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE card_foils SET front_key = $2, mask_key = $3, etch_key = NULLIF($4, ''), copied_hash = $5 WHERE card_id = $1`,
		cardID, frontKey, maskKey, etchKey, hash)
	return wrap("set foil keys", err)
}

// FoillessCards lists the cards of the given sets without foil.
func (r *Repository) FoillessCards(ctx context.Context, setIDs []string) ([]catalog.FoilCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.set_id, c.local_id, c.rarity
		FROM cards c
		WHERE c.set_id = ANY($1) AND NOT EXISTS (SELECT 1 FROM card_foils f WHERE f.card_id = c.id)
		ORDER BY c.id`, setIDs)
	if err != nil {
		return nil, wrap("list cards without foil", err)
	}
	defer rows.Close()

	var cards []catalog.FoilCandidate
	for rows.Next() {
		var c catalog.FoilCandidate
		if err := rows.Scan(&c.CardID, &c.SetID, &c.LocalID, &c.Rarity); err != nil {
			return nil, wrap("scan card without foil", err)
		}
		cards = append(cards, c)
	}
	return cards, wrap("list cards without foil", rows.Err())
}
