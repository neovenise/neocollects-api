package store

import (
	"context"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// UpsertBlock inserts a block or updates its name and position.
func (r *Repository) UpsertBlock(ctx context.Context, b catalog.Block) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO blocks (id, name, position, is_digital) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, position = EXCLUDED.position, is_digital = EXCLUDED.is_digital`,
		b.ID, b.Name, b.Position, b.Digital)
	return wrap("upsert block", err)
}

// EnsureBlock inserts a block only if it does not exist.
func (r *Repository) EnsureBlock(ctx context.Context, b catalog.Block) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO blocks (id, name, position, is_digital) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`,
		b.ID, b.Name, b.Position, b.Digital)
	return wrap("ensure block", err)
}

// UpsertSet inserts or updates a set.
func (r *Repository) UpsertSet(ctx context.Context, s catalog.Set) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sets (id, code, name, block_id, official_count, total_count, release_date, logo_source_url, symbol_source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''))
		ON CONFLICT (id) DO UPDATE SET
			code = EXCLUDED.code, name = EXCLUDED.name, block_id = EXCLUDED.block_id,
			official_count = EXCLUDED.official_count, total_count = EXCLUDED.total_count,
			release_date = EXCLUDED.release_date,
			logo_source_url = EXCLUDED.logo_source_url, symbol_source_url = EXCLUDED.symbol_source_url`,
		s.ID, s.Code, s.Name, s.BlockID, s.OfficialCount, s.TotalCount, s.ReleaseDate, s.LogoSourceURL, s.SymbolSourceURL)
	return wrap("upsert set", err)
}

// UpsertCard inserts or updates a card.
func (r *Repository) UpsertCard(ctx context.Context, c catalog.Card) error {
	var number *int
	if n, ok := catalog.LocalNumber(c.LocalID); ok {
		number = &n
	}
	types := c.Types
	if types == nil {
		types = []string{}
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity, source_image_url,
		                   category, types, hp, stage, trainer_type, regulation_mark, illustrator)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			set_id = EXCLUDED.set_id, local_id = EXCLUDED.local_id, local_number = EXCLUDED.local_number,
			name = EXCLUDED.name, name_search = EXCLUDED.name_search,
			rarity = CASE WHEN EXCLUDED.rarity = '' THEN cards.rarity ELSE EXCLUDED.rarity END,
			source_image_url = COALESCE(EXCLUDED.source_image_url, cards.source_image_url),
			category = CASE WHEN EXCLUDED.category = '' THEN cards.category ELSE EXCLUDED.category END,
			types = CASE WHEN cardinality(EXCLUDED.types) = 0 THEN cards.types ELSE EXCLUDED.types END,
			hp = COALESCE(EXCLUDED.hp, cards.hp),
			stage = CASE WHEN EXCLUDED.stage = '' THEN cards.stage ELSE EXCLUDED.stage END,
			trainer_type = CASE WHEN EXCLUDED.trainer_type = '' THEN cards.trainer_type ELSE EXCLUDED.trainer_type END,
			regulation_mark = CASE WHEN EXCLUDED.regulation_mark = '' THEN cards.regulation_mark ELSE EXCLUDED.regulation_mark END,
			illustrator = CASE WHEN EXCLUDED.illustrator = '' THEN cards.illustrator ELSE EXCLUDED.illustrator END`,
		c.ID, c.SetID, c.LocalID, number, c.Name, catalog.SearchName(c.Name), c.Rarity, c.SourceImageURL,
		c.Category, types, c.HP, c.Stage, c.TrainerType, c.RegulationMark, c.Illustrator)
	return wrap("upsert card", err)
}

// UpsertPrice stores a card's price and records changes in history.
func (r *Repository) UpsertPrice(ctx context.Context, p catalog.Price) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO card_prices (card_id, cents, currency, updated_at, avg1_cents, avg7_cents, avg30_cents, product_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (card_id) DO UPDATE SET cents = EXCLUDED.cents, currency = EXCLUDED.currency,
			updated_at = EXCLUDED.updated_at, avg1_cents = EXCLUDED.avg1_cents, avg7_cents = EXCLUDED.avg7_cents,
			avg30_cents = EXCLUDED.avg30_cents, product_id = EXCLUDED.product_id`,
		p.CardID, p.Cents, p.Currency, p.UpdatedAt, p.Avg1Cents, p.Avg7Cents, p.Avg30Cents, p.ProductID); err != nil {
		return wrap("upsert price", err)
	}

	day := p.UpdatedAt.UTC().Format(time.DateOnly)
	_, err := r.db.Exec(ctx, `
		INSERT INTO card_price_history (card_id, day, cents)
		SELECT $1, $2::date, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM card_price_history h
			WHERE h.card_id = $1 AND h.cents = $3
			  AND h.day = (SELECT max(day) FROM card_price_history WHERE card_id = $1))
		ON CONFLICT (card_id, day) DO UPDATE SET cents = EXCLUDED.cents`,
		p.CardID, day, p.Cents)
	return wrap("append price history", err)
}

// CardIDs returns the ids of a set's stored cards.
func (r *Repository) CardIDs(ctx context.Context, setID string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM cards WHERE set_id = $1`, setID)
	if err != nil {
		return nil, wrap("list cards", err)
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, wrap("scan card id", err)
		}
		ids[id] = true
	}
	return ids, wrap("list cards", rows.Err())
}

// SetCount returns how many sets are stored.
func (r *Repository) SetCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM sets`).Scan(&n)
	return n, wrap("count sets", err)
}
