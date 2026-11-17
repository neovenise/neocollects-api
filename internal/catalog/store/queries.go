package store

import (
	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/catalog"
)

type Queries struct {
	db DBTX
}

func NewQueries(db DBTX) *Queries {
	return &Queries{db: db}
}

const setColumns = `s.id, s.code, s.name, s.block_id, b.name, s.official_count, s.total_count, s.release_date,
	s.logo_key, s.symbol_key`

const cardColumns = `c.id, c.set_id, c.local_id, c.name, c.rarity, c.image_key, c.thumb_key`

func scanSet(row pgx.Row) (catalog.SetRow, error) {
	var s catalog.SetRow
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.BlockID, &s.BlockName, &s.OfficialCount, &s.TotalCount,
		&s.ReleaseDate, &s.LogoKey, &s.SymbolKey)
	return s, err
}

func scanCard(row pgx.Row) (catalog.CardRow, error) {
	var c catalog.CardRow
	err := row.Scan(&c.ID, &c.SetID, &c.LocalID, &c.Name, &c.Rarity, &c.ImageKey, &c.ThumbKey)
	return c, err
}
