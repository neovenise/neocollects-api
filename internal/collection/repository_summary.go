package collection

import (
	"context"

	"github.com/neovenise/neocollects-api/internal/catalog/store"
)

// Summary computes the collection's totals with today's prices.
func (r *Repository) Summary(ctx context.Context, userID string) (Summary, error) {
	s := Summary{Sets: []SetCount{}}

	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(e.quantity), 0),
		       COUNT(DISTINCT e.card_id),
		       COALESCE(SUM(e.quantity::bigint * p.cents), 0),
		       COUNT(DISTINCT e.card_id) FILTER (WHERE p.cents IS NULL)
		FROM collection_entries e
		JOIN cards c ON c.id = e.card_id
		LEFT JOIN card_prices p ON p.card_id = c.id AND `+store.PriceIsItsOwnSQL+`
		WHERE e.user_id = $1`, userID).Scan(&s.TotalQuantity, &s.DistinctCards, &s.ValueCents, &s.UnpricedCards)
	if err != nil {
		return Summary{}, wrap("summary totals", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT c.set_id, COUNT(DISTINCT e.card_id)
		FROM collection_entries e JOIN cards c ON c.id = e.card_id
		WHERE e.user_id = $1
		GROUP BY c.set_id ORDER BY c.set_id`, userID)
	if err != nil {
		return Summary{}, wrap("summary sets", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc SetCount
		if err := rows.Scan(&sc.SetID, &sc.OwnedCount); err != nil {
			return Summary{}, wrap("scan set count", err)
		}
		s.Sets = append(s.Sets, sc)
	}
	return s, wrap("summary sets", rows.Err())
}
