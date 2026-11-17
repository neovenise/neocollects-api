package collection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Add inserts an entry or raises its quantity, atomically.
func (r *Repository) Add(ctx context.Context, userID string, in NewEntry, idem *Idempotency) (AddResult, error) {
	if idem == nil {
		return r.add(ctx, r.db, userID, in)
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AddResult{}, wrap("begin", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID+":"+idem.Key); err != nil {
		return AddResult{}, wrap("lock idempotency key", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND created_at < now() - $2::interval`,
		userID, fmt.Sprintf("%d seconds", int(idempotencyTTL.Seconds()))); err != nil {
		return AddResult{}, wrap("expire idempotency keys", err)
	}

	var storedHash []byte
	var created bool
	var stored []byte
	err = tx.QueryRow(ctx, `SELECT request_hash, created, entry FROM idempotency_keys WHERE user_id = $1 AND key = $2`,
		userID, idem.Key).Scan(&storedHash, &created, &stored)
	switch {
	case err == nil:
		if string(storedHash) != string(idem.RequestHash) {
			return AddResult{}, ErrKeyReused
		}
		var entry Entry
		if err := json.Unmarshal(stored, &entry); err != nil {
			return AddResult{}, wrap("decode stored answer", err)
		}
		return AddResult{Entry: entry, Created: created, Replayed: true}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return AddResult{}, wrap("read idempotency key", err)
	}

	result, err := r.add(ctx, tx, userID, in)
	if err != nil {
		return AddResult{}, err
	}
	body, err := json.Marshal(result.Entry)
	if err != nil {
		return AddResult{}, wrap("encode answer", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO idempotency_keys (user_id, key, request_hash, created, entry) VALUES ($1, $2, $3, $4, $5)`,
		userID, idem.Key, idem.RequestHash, result.Created, string(body)); err != nil {
		return AddResult{}, wrap("store idempotency key", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AddResult{}, wrap("commit", err)
	}
	return result, nil
}

func (r *Repository) add(ctx context.Context, db DB, userID string, in NewEntry) (AddResult, error) {

	var known bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM cards c JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
		               WHERE c.id = $1 AND NOT b.is_digital)`, in.CardID).Scan(&known); err != nil {
		return AddResult{}, wrap("check card", err)
	}
	if !known {
		return AddResult{}, ErrCardNotFound
	}

	var e Entry
	var created bool
	err := db.QueryRow(ctx, `
		INSERT INTO collection_entries AS e (user_id, card_id, condition, language, quantity)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, card_id, condition, language)
		DO UPDATE SET quantity = e.quantity + EXCLUDED.quantity
		WHERE e.quantity + EXCLUDED.quantity <= `+fmt.Sprint(MaxQuantity)+`
		RETURNING `+entryColumns+`, (xmax = 0)`,
		userID, in.CardID, in.Condition, in.Language, in.Quantity).
		Scan(&e.ID, &e.CardID, &e.Condition, &e.Language, &e.Quantity, &e.AddedAt, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return AddResult{}, ErrQuantityTooLarge
	}
	e.AddedAt = e.AddedAt.UTC()
	return AddResult{Entry: e, Created: created}, wrap("add entry", err)
}
