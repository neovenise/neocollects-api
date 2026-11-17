package lists

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxCustomLists = 100

	MaxCardsPerList = 5000

	MaxCardQuantity = 99

	previewSize = 3
)

var (
	ErrListNotFound = errors.New("lists: list not found")

	ErrDefaultList  = errors.New("lists: default list")
	ErrCardNotFound = errors.New("lists: card not found")
	ErrTooManyLists = errors.New("lists: too many lists")
	ErrListFull     = errors.New("lists: list is full")
)

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ListCard struct {
	CardID   string `json:"cardId"`
	Quantity int    `json:"quantity"`
}

type List struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	CardCount      int        `json:"cardCount"`
	PreviewCardIDs []string   `json:"previewCardIds"`
	Cards          []ListCard `json:"cards,omitempty"`
}

type Repository struct{ db DB }

func NewRepository(db DB) *Repository { return &Repository{db: db} }

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("lists: %s: %w", op, err)
}

const listOrder = `ORDER BY CASE l.kind WHEN 'favorites' THEN 0 WHEN 'trade' THEN 1 WHEN 'wishlist' THEN 2 ELSE 3 END, l.created_at, l.id`

func (r *Repository) selectLists(ctx context.Context, withCards bool, where string, args ...any) ([]List, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id::text, l.kind, l.name,
		       (SELECT count(*) FROM list_cards WHERE list_id = l.id),
		       (SELECT COALESCE(array_agg(card_id ORDER BY position), '{}') FROM list_cards WHERE list_id = l.id),
		       (SELECT COALESCE(array_agg(quantity ORDER BY position), '{}') FROM list_cards WHERE list_id = l.id)
		FROM lists l WHERE `+where+` `+listOrder, args...)
	if err != nil {
		return nil, wrap("read lists", err)
	}
	defer rows.Close()
	out := []List{}
	for rows.Next() {
		var l List
		var ids []string
		var quantities []int32
		if err := rows.Scan(&l.ID, &l.Kind, &l.Name, &l.CardCount, &ids, &quantities); err != nil {
			return nil, wrap("scan list", err)
		}
		l.PreviewCardIDs = ids[:min(previewSize, len(ids))]
		if withCards {
			l.Cards = make([]ListCard, len(ids))
			for i, id := range ids {
				l.Cards[i] = ListCard{CardID: id, Quantity: int(quantities[i])}
			}
		}
		out = append(out, l)
	}
	return out, wrap("read lists", rows.Err())
}

// Lists returns the user's lists, default ones first.
func (r *Repository) Lists(ctx context.Context, userID string, withCards bool) ([]List, error) {
	return r.selectLists(ctx, withCards, `l.user_id = $1`, userID)
}

func (r *Repository) one(ctx context.Context, userID, listID string) (List, error) {
	found, err := r.selectLists(ctx, false, `l.user_id = $1 AND l.id = $2`, userID, listID)
	if err != nil {
		return List{}, err
	}
	if len(found) == 0 {
		return List{}, ErrListNotFound
	}
	return found[0], nil
}

// Create adds a list for the user.
func (r *Repository) Create(ctx context.Context, userID, name string) (List, error) {
	var id string

	err := r.db.QueryRow(ctx, `
		INSERT INTO lists (user_id, kind, name)
		SELECT $1, 'custom', $2
		WHERE (SELECT count(*) FROM lists WHERE user_id = $1 AND kind = 'custom') < $3
		RETURNING id::text`, userID, name, MaxCustomLists).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return List{}, ErrTooManyLists
	}
	if err != nil {
		return List{}, wrap("create list", err)
	}
	return r.one(ctx, userID, id)
}

func (r *Repository) kindOf(ctx context.Context, userID, listID string) (string, error) {
	var kind string
	err := r.db.QueryRow(ctx, `SELECT kind FROM lists WHERE id = $1 AND user_id = $2`, listID, userID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrListNotFound
	}
	return kind, wrap("read list", err)
}

// Rename changes the name of one of the user's lists.
func (r *Repository) Rename(ctx context.Context, userID, listID, name string) (List, error) {
	kind, err := r.kindOf(ctx, userID, listID)
	if err != nil {
		return List{}, err
	}
	if kind != "custom" {
		return List{}, ErrDefaultList
	}
	if _, err := r.db.Exec(ctx, `UPDATE lists SET name = $3 WHERE id = $1 AND user_id = $2`, listID, userID, name); err != nil {
		return List{}, wrap("rename list", err)
	}
	return r.one(ctx, userID, listID)
}

// Delete removes a list and its cards.
func (r *Repository) Delete(ctx context.Context, userID, listID string) error {
	kind, err := r.kindOf(ctx, userID, listID)
	if err != nil {
		return err
	}
	if kind != "custom" {
		return ErrDefaultList
	}
	_, err = r.db.Exec(ctx, `DELETE FROM lists WHERE id = $1 AND user_id = $2`, listID, userID)
	return wrap("delete list", err)
}

// PutCard adds a card to a list.
func (r *Repository) PutCard(ctx context.Context, userID, listID, cardID string, quantity *int) error {
	if _, err := r.kindOf(ctx, userID, listID); err != nil {
		return err
	}

	var known bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM cards c JOIN sets s ON s.id = c.set_id JOIN blocks b ON b.id = s.block_id
		               WHERE c.id = $1 AND NOT b.is_digital)`, cardID).Scan(&known); err != nil {
		return wrap("check card", err)
	}
	if !known {
		return ErrCardNotFound
	}
	var count int
	var already bool
	if err := r.db.QueryRow(ctx, `SELECT count(*), COALESCE(bool_or(card_id = $2), false) FROM list_cards WHERE list_id = $1`,
		listID, cardID).Scan(&count, &already); err != nil {
		return wrap("count list", err)
	}
	if !already && count >= MaxCardsPerList {
		return ErrListFull
	}
	if quantity == nil && already {
		return nil
	}
	want := 1
	if quantity != nil {
		want = *quantity
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO list_cards (list_id, card_id, position, quantity)
		SELECT $1, $2, COALESCE(MAX(position), 0) + 1, $3 FROM list_cards WHERE list_id = $1
		ON CONFLICT (list_id, card_id) DO UPDATE SET quantity = EXCLUDED.quantity`, listID, cardID, want)
	return wrap("put card", err)
}

// RemoveCard takes a card out of a list.
func (r *Repository) RemoveCard(ctx context.Context, userID, listID, cardID string) error {
	if _, err := r.kindOf(ctx, userID, listID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM list_cards WHERE list_id = $1 AND card_id = $2`, listID, cardID)
	return wrap("remove card", err)
}
