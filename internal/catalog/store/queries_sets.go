package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/catalog"
)

// Blocks lists the blocks, newest first.
func (q *Queries) Blocks(ctx context.Context) ([]catalog.BlockRow, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name FROM blocks WHERE NOT is_digital ORDER BY position`)
	if err != nil {
		return nil, wrap("list blocks", err)
	}
	defer rows.Close()

	blocks := []catalog.BlockRow{}
	for rows.Next() {
		var b catalog.BlockRow
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, wrap("scan block", err)
		}
		blocks = append(blocks, b)
	}
	return blocks, wrap("list blocks", rows.Err())
}

// Sets lists the sets by block, newest first.
func (q *Queries) Sets(ctx context.Context, f catalog.SetFilter) ([]catalog.SetRow, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+setColumns+`
		FROM sets s JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital
		  AND ($1 = '' OR s.block_id = $1)
		  AND ($2::int IS NULL OR s.official_count = $2)
		ORDER BY b.position, s.release_date DESC NULLS LAST, s.id`,
		f.BlockID, f.OfficialCount)
	if err != nil {
		return nil, wrap("list sets", err)
	}
	defer rows.Close()

	sets := []catalog.SetRow{}
	for rows.Next() {
		s, err := scanSet(rows)
		if err != nil {
			return nil, wrap("scan set", err)
		}
		sets = append(sets, s)
	}
	return sets, wrap("list sets", rows.Err())
}

// Set returns one set, or ErrNotFound.
func (q *Queries) Set(ctx context.Context, id string) (catalog.SetRow, error) {
	s, err := scanSet(q.db.QueryRow(ctx, `
		SELECT `+setColumns+`
		FROM sets s JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital AND s.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.SetRow{}, catalog.ErrNotFound
	}
	return s, wrap("get set", err)
}

// SetCards returns a set's cards in printed order.
func (q *Queries) SetCards(ctx context.Context, setID string) ([]catalog.CardRow, error) {
	if _, err := q.Set(ctx, setID); err != nil {
		return nil, err
	}
	rows, err := q.db.Query(ctx, `
		SELECT `+cardColumns+`
		FROM cards c WHERE c.set_id = $1
		ORDER BY c.local_number NULLS LAST, c.local_id`, setID)
	if err != nil {
		return nil, wrap("list set cards", err)
	}
	defer rows.Close()
	return collectCards(rows)
}

func collectCards(rows pgx.Rows) ([]catalog.CardRow, error) {
	cards := []catalog.CardRow{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, wrap("scan card", err)
		}
		cards = append(cards, c)
	}
	return cards, wrap("list cards", rows.Err())
}
