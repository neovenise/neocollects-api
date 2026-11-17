package collection

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Update applies a patch to one of the user's entries.
func (r *Repository) Update(ctx context.Context, userID, entryID string, p Patch) (Entry, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Entry{}, wrap("begin", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanEntry(tx.QueryRow(ctx,
		`SELECT `+entryColumns+` FROM collection_entries e WHERE e.id = $1 AND e.user_id = $2 FOR UPDATE`, entryID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrEntryNotFound
	}
	if err != nil {
		return Entry{}, wrap("read entry", err)
	}

	condition, language, quantity := current.Condition, current.Language, current.Quantity
	if p.Condition != nil {
		condition = *p.Condition
	}
	if p.Language != nil {
		language = *p.Language
	}
	if p.Quantity != nil {
		quantity = *p.Quantity
	}

	var result Entry
	if condition == current.Condition && language == current.Language {
		result, err = scanEntry(tx.QueryRow(ctx, `
			UPDATE collection_entries e SET quantity = $3 WHERE e.id = $1 AND e.user_id = $2 RETURNING `+entryColumns,
			entryID, userID, quantity))
		if err != nil {
			return Entry{}, wrap("update entry", err)
		}
	} else {
		var targetID string
		var targetQuantity int
		err := tx.QueryRow(ctx, `
			SELECT id::text, quantity FROM collection_entries
			WHERE user_id = $1 AND card_id = $2 AND condition = $3 AND language = $4 FOR UPDATE`,
			userID, current.CardID, condition, language).Scan(&targetID, &targetQuantity)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			result, err = scanEntry(tx.QueryRow(ctx, `
				UPDATE collection_entries e SET condition = $3, language = $4, quantity = $5
				WHERE e.id = $1 AND e.user_id = $2 RETURNING `+entryColumns,
				entryID, userID, condition, language, quantity))
			if err != nil {
				return Entry{}, wrap("move entry", err)
			}
		case err != nil:
			return Entry{}, wrap("read merge target", err)
		default:
			if targetQuantity+quantity > MaxQuantity {
				return Entry{}, ErrQuantityTooLarge
			}
			result, err = scanEntry(tx.QueryRow(ctx, `
				UPDATE collection_entries e SET quantity = $3 WHERE e.id = $1 AND e.user_id = $2 RETURNING `+entryColumns,
				targetID, userID, targetQuantity+quantity))
			if err != nil {
				return Entry{}, wrap("merge entry", err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM collection_entries WHERE id = $1 AND user_id = $2`, entryID, userID); err != nil {
				return Entry{}, wrap("remove merged entry", err)
			}
		}
	}
	return result, wrap("commit", tx.Commit(ctx))
}
