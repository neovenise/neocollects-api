package collection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct{ db DB }

func NewRepository(db DB) *Repository { return &Repository{db: db} }

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("collection: %s: %w", op, err)
}

const entryColumns = `e.id::text, e.card_id, e.condition, e.language, e.quantity, e.added_at`

func scanEntry(row pgx.Row) (Entry, error) {
	var e Entry
	err := row.Scan(&e.ID, &e.CardID, &e.Condition, &e.Language, &e.Quantity, &e.AddedAt)
	e.AddedAt = e.AddedAt.UTC()
	return e, err
}

// Delete removes one of the user's entries.
func (r *Repository) Delete(ctx context.Context, userID, entryID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM collection_entries WHERE id = $1 AND user_id = $2`, entryID, userID)
	if err != nil {
		return wrap("delete entry", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEntryNotFound
	}
	return nil
}

// List returns a page of entries, newest first.
func (r *Repository) List(ctx context.Context, userID, setID string, limit, offset int) (entries []Entry, hasMore bool, err error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+entryColumns+`
		FROM collection_entries e JOIN cards c ON c.id = e.card_id
		WHERE e.user_id = $1 AND ($2 = '' OR c.set_id = $2)
		ORDER BY e.added_at DESC, e.id
		LIMIT $3 OFFSET $4`, userID, setID, limit+1, offset)
	if err != nil {
		return nil, false, wrap("list entries", err)
	}
	defer rows.Close()
	entries = []Entry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, false, wrap("scan entry", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, false, wrap("list entries", err)
	}
	if len(entries) > limit {
		return entries[:limit], true, nil
	}
	return entries, false, nil
}
