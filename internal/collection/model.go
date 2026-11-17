package collection

import (
	"errors"
	"time"
)

const MaxQuantity = 9999

const idempotencyTTL = 24 * time.Hour

var (
	ErrCardNotFound = errors.New("collection: card not found")

	ErrEntryNotFound = errors.New("collection: entry not found")

	ErrQuantityTooLarge = errors.New("collection: quantity too large")

	ErrKeyReused = errors.New("collection: idempotency key reused for another request")
)

type Entry struct {
	ID        string    `json:"id"`
	CardID    string    `json:"cardId"`
	Condition string    `json:"condition"`
	Language  string    `json:"language"`
	Quantity  int       `json:"quantity"`
	AddedAt   time.Time `json:"addedAt"`
}

type NewEntry struct {
	CardID    string
	Condition string
	Language  string
	Quantity  int
}

type Patch struct {
	Quantity  *int
	Condition *string
	Language  *string
}

type Idempotency struct {
	Key         string
	RequestHash []byte
}

type AddResult struct {
	Entry    Entry
	Created  bool
	Replayed bool
}

type Summary struct {
	TotalQuantity int        `json:"totalQuantity"`
	DistinctCards int        `json:"distinctCards"`
	ValueCents    int64      `json:"valueCents"`
	UnpricedCards int        `json:"unpricedCards"`
	Sets          []SetCount `json:"sets"`
}

type SetCount struct {
	SetID      string `json:"setId"`
	OwnedCount int    `json:"ownedCount"`
}
