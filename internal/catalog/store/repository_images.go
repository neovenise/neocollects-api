package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/catalog"
)

// CardIDsWithoutImage lists a set's cards that have no picture.
func (r *Repository) CardIDsWithoutImage(ctx context.Context, setID string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM cards WHERE set_id = $1 AND source_image_url IS NULL AND sharp_image_url IS NULL`, setID)
	if err != nil {
		return nil, wrap("list cards without image", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, wrap("scan card id", err)
		}
		ids[id] = true
	}
	return ids, wrap("list cards without image", rows.Err())
}

// SetFallbackImage gives a picture-less card another language's picture.
func (r *Repository) SetFallbackImage(ctx context.Context, cardID, url string) error {
	_, err := r.db.Exec(ctx, `UPDATE cards SET source_image_url = $2 WHERE id = $1 AND source_image_url IS NULL AND sharp_image_url IS NULL`, cardID, url)
	return wrap("set fallback image", err)
}

// SetSharpImage stores a card's high-resolution picture address.
func (r *Repository) SetSharpImage(ctx context.Context, cardID, url string) error {
	_, err := r.db.Exec(ctx, `UPDATE cards SET sharp_image_url = $2 WHERE id = $1 AND sharp_image_url IS DISTINCT FROM $2`, cardID, url)
	return wrap("set sharp image", err)
}

// CardsForMirroring lists cards with a picture and their last copy.
func (r *Repository) CardsForMirroring(ctx context.Context) ([]catalog.MirrorCard, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, COALESCE(c.source_image_url, ''), COALESCE(c.sharp_image_url, ''), COALESCE(c.image_source_hash, '')
		FROM cards c JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital AND (c.source_image_url IS NOT NULL OR c.sharp_image_url IS NOT NULL)
		ORDER BY c.id`)
	if err != nil {
		return nil, wrap("list cards to mirror", err)
	}
	defer rows.Close()

	var cards []catalog.MirrorCard
	for rows.Next() {
		var c catalog.MirrorCard
		if err := rows.Scan(&c.ID, &c.SourceURL, &c.SharpURL, &c.Hash); err != nil {
			return nil, wrap("scan card to mirror", err)
		}
		cards = append(cards, c)
	}
	return cards, wrap("list cards to mirror", rows.Err())
}

// SetImageKeys records where a card's copied pictures are stored.
func (r *Repository) SetImageKeys(ctx context.Context, cardID, imageKey, thumbKey, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE cards SET image_key = $2, thumb_key = $3, image_source_hash = $4 WHERE id = $1`, cardID, imageKey, thumbKey, hash)
	return wrap("set image keys", err)
}

// CardImageHash returns the source hash of a card's copied picture.
func (r *Repository) CardImageHash(ctx context.Context, cardID string) (hash string, exists bool, err error) {
	err = r.db.QueryRow(ctx, `SELECT COALESCE(image_source_hash, '') FROM cards WHERE id = $1`, cardID).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, wrap("read image hash", err)
	}
	return hash, true, nil
}

// CardsWithoutPicture lists a set's cards that have no picture.
func (r *Repository) CardsWithoutPicture(ctx context.Context, setID string) ([]catalog.FoilCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, set_id, local_id, rarity FROM cards
		WHERE set_id = $1 AND image_key IS NULL AND source_image_url IS NULL AND sharp_image_url IS NULL
		ORDER BY id`, setID)
	if err != nil {
		return nil, wrap("list cards without picture", err)
	}
	defer rows.Close()

	var cards []catalog.FoilCandidate
	for rows.Next() {
		var c catalog.FoilCandidate
		if err := rows.Scan(&c.CardID, &c.SetID, &c.LocalID, &c.Rarity); err != nil {
			return nil, wrap("scan card without picture", err)
		}
		cards = append(cards, c)
	}
	return cards, wrap("list cards without picture", rows.Err())
}
