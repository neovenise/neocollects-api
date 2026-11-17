package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/catalog"
)

// PriceIsItsOwnSQL drops prices copied from another card of the set.
const PriceIsItsOwnSQL = `NOT EXISTS (
	SELECT 1 FROM card_prices other JOIN cards oc ON oc.id = other.card_id
	WHERE other.product_id = p.product_id AND oc.set_id = c.set_id AND oc.id <> c.id
	  AND (oc.local_number < c.local_number OR (oc.local_number = c.local_number AND oc.id < c.id))
	  AND (oc.name <> c.name OR oc.rarity <> c.rarity))`

// Card returns one card with its set and price.
func (q *Queries) Card(ctx context.Context, id string) (catalog.CardDetailRow, error) {
	var d catalog.CardDetailRow
	err := q.db.QueryRow(ctx, `
		SELECT `+cardColumns+`, `+setColumns+`, c.illustrator, p.cents, p.currency, p.updated_at, p.avg1_cents, p.avg7_cents, p.avg30_cents
		FROM cards c
		JOIN sets s ON s.id = c.set_id
		JOIN blocks b ON b.id = s.block_id
		LEFT JOIN card_prices p ON p.card_id = c.id AND `+PriceIsItsOwnSQL+`
		WHERE NOT b.is_digital AND c.id = $1`, id).Scan(
		&d.ID, &d.SetID, &d.LocalID, &d.Name, &d.Rarity, &d.ImageKey, &d.ThumbKey,
		&d.Set.ID, &d.Set.Code, &d.Set.Name, &d.Set.BlockID, &d.Set.BlockName, &d.Set.OfficialCount, &d.Set.TotalCount,
		&d.Set.ReleaseDate, &d.Set.LogoKey, &d.Set.SymbolKey, &d.Illustrator,
		&d.PriceCents, &d.PriceCurrency, &d.PriceUpdatedAt, &d.PriceAvg1Cents, &d.PriceAvg7Cents, &d.PriceAvg30Cents)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.CardDetailRow{}, catalog.ErrNotFound
	}
	return d, wrap("get card", err)
}

// Foil returns a card's foil, or ErrNotFound.
func (q *Queries) Foil(ctx context.Context, cardID string) (catalog.FoilRow, error) {
	var f catalog.FoilRow
	err := q.db.QueryRow(ctx, `
		SELECT f.look, f.front_key, f.mask_key, f.etch_key
		FROM card_foils f
		JOIN cards c ON c.id = f.card_id
		JOIN sets s ON s.id = c.set_id
		JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital AND f.card_id = $1`, cardID).Scan(&f.Look, &f.FrontKey, &f.MaskKey, &f.EtchKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.FoilRow{}, catalog.ErrNotFound
	}
	return f, wrap("get foil", err)
}

// PriceHistory returns a card's price points, oldest first.
func (q *Queries) PriceHistory(ctx context.Context, cardID string, from, to *string) ([]catalog.PricePointRow, error) {
	if _, err := q.Card(ctx, cardID); err != nil {
		return nil, err
	}
	rows, err := q.db.Query(ctx, `
		SELECT h.day::text, h.cents FROM card_price_history h
		WHERE h.card_id = $1 AND ($2::date IS NULL OR h.day >= $2::date) AND ($3::date IS NULL OR h.day <= $3::date)
		  AND EXISTS (SELECT 1 FROM cards c JOIN card_prices p ON p.card_id = c.id WHERE c.id = h.card_id AND `+PriceIsItsOwnSQL+`)
		ORDER BY h.day`, cardID, from, to)
	if err != nil {
		return nil, wrap("price history", err)
	}
	defer rows.Close()

	points := []catalog.PricePointRow{}
	for rows.Next() {
		var p catalog.PricePointRow
		if err := rows.Scan(&p.Day, &p.Cents); err != nil {
			return nil, wrap("scan price point", err)
		}
		points = append(points, p)
	}
	return points, wrap("price history", rows.Err())
}
