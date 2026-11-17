package store

import (
	"context"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// Snapshot returns every physical set and card, by id.
func (q *Queries) Snapshot(ctx context.Context) ([]catalog.SnapshotSet, []catalog.SnapshotCard, error) {
	setRows, err := q.db.Query(ctx, `
		SELECT s.id, s.code, s.name, b.name, s.official_count, s.total_count
		FROM sets s JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital ORDER BY s.id`)
	if err != nil {
		return nil, nil, wrap("snapshot sets", err)
	}
	defer setRows.Close()
	sets := []catalog.SnapshotSet{}
	for setRows.Next() {
		var s catalog.SnapshotSet
		if err := setRows.Scan(&s.ID, &s.Code, &s.Name, &s.BlockName, &s.OfficialCount, &s.TotalCount); err != nil {
			return nil, nil, wrap("scan snapshot set", err)
		}
		sets = append(sets, s)
	}
	if err := setRows.Err(); err != nil {
		return nil, nil, wrap("snapshot sets", err)
	}

	cardRows, err := q.db.Query(ctx, `
		SELECT c.id, c.set_id, c.local_id, c.name, c.rarity
		FROM cards c JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital ORDER BY c.id`)
	if err != nil {
		return nil, nil, wrap("snapshot cards", err)
	}
	defer cardRows.Close()
	cards := []catalog.SnapshotCard{}
	for cardRows.Next() {
		var c catalog.SnapshotCard
		if err := cardRows.Scan(&c.ID, &c.SetID, &c.LocalID, &c.Name, &c.Rarity); err != nil {
			return nil, nil, wrap("scan snapshot card", err)
		}
		cards = append(cards, c)
	}
	return sets, cards, wrap("snapshot cards", cardRows.Err())
}
