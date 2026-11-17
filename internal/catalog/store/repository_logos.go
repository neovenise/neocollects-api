package store

import (
	"context"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

// SetsWithoutLogoCopy lists sets whose logo or symbol is not in our storage.
func (r *Repository) SetsWithoutLogoCopy(ctx context.Context) ([]catalog.LogoCandidate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.code, COALESCE(s.logo_source_url, ''), COALESCE(s.symbol_source_url, ''),
			s.logo_key IS NULL, s.symbol_key IS NULL AND COALESCE(s.symbol_source_url, '') <> ''
		FROM sets s JOIN blocks b ON b.id = s.block_id
		WHERE NOT b.is_digital
			AND (s.logo_key IS NULL OR (s.symbol_key IS NULL AND COALESCE(s.symbol_source_url, '') <> ''))
		ORDER BY s.id`)
	if err != nil {
		return nil, wrap("list sets without a logo copy", err)
	}
	defer rows.Close()

	var sets []catalog.LogoCandidate
	for rows.Next() {
		var c catalog.LogoCandidate
		if err := rows.Scan(&c.ID, &c.Code, &c.LogoSource, &c.SymbolSource, &c.NeedsLogo, &c.NeedsSymbol); err != nil {
			return nil, wrap("scan set", err)
		}
		sets = append(sets, c)
	}
	return sets, wrap("list sets without a logo copy", rows.Err())
}

// SetLogoKey records where a set's logo is stored.
func (r *Repository) SetLogoKey(ctx context.Context, setID, key string) error {
	_, err := r.db.Exec(ctx, `UPDATE sets SET logo_key = $2 WHERE id = $1`, setID, key)
	return wrap("set logo key", err)
}

// SetSymbolKey records where a set's symbol is stored.
func (r *Repository) SetSymbolKey(ctx context.Context, setID, key string) error {
	_, err := r.db.Exec(ctx, `UPDATE sets SET symbol_key = $2 WHERE id = $1`, setID, key)
	return wrap("set symbol key", err)
}
