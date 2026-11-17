package collection

import (
	"errors"
	"testing"
)

func TestAddingTheSameCopyRaisesTheQuantityInsteadOfCreatingASecondEntry(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	first := mustAdd(t, r, user, entry(alpha, "nearMint", "FR", 1))
	second := mustAdd(t, r, user, entry(alpha, "nearMint", "FR", 2))

	if !first.Created || second.Created {
		t.Errorf("created = %v then %v, want true then false", first.Created, second.Created)
	}
	if second.Entry.ID != first.Entry.ID || second.Entry.Quantity != 3 {
		t.Errorf("merged entry = %+v, want the first one with quantity 3", second.Entry)
	}
	if !second.Entry.AddedAt.Equal(first.Entry.AddedAt) {
		t.Error("a merge must keep the date the card was first added")
	}
}

func TestAnotherConditionOrLanguageIsAnotherEntry(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	a := mustAdd(t, r, user, entry(alpha, "nearMint", "FR", 1))
	b := mustAdd(t, r, user, entry(alpha, "played", "FR", 1))
	c := mustAdd(t, r, user, entry(alpha, "nearMint", "EN", 1))

	if a.Entry.ID == b.Entry.ID || a.Entry.ID == c.Entry.ID || b.Entry.ID == c.Entry.ID || !b.Created || !c.Created {
		t.Errorf("entries = %+v %+v %+v, want three distinct ones", a.Entry, b.Entry, c.Entry)
	}
}

func TestACardThatDoesNotExistOrIsDigitalOnlyCannotBeAdded(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	for _, card := range []string{"nope-1", digital} {
		if _, err := r.Add(ctx(), user, entry(card, "mint", "FR", 1), nil); !errors.Is(err, ErrCardNotFound) {
			t.Errorf("add %q: err = %v, want ErrCardNotFound", card, err)
		}
	}
}

func TestMergingOverTheMaximumIsRefusedAndLeavesTheEntryAndTheTransactionUsable(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustAdd(t, r, user, entry(alpha, "mint", "FR", MaxQuantity-1))

	_, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 2), nil)

	if !errors.Is(err, ErrQuantityTooLarge) {
		t.Fatalf("err = %v, want ErrQuantityTooLarge", err)
	}

	got := mustAdd(t, r, user, entry(alpha, "mint", "FR", 1))
	if got.Entry.Quantity != MaxQuantity {
		t.Errorf("quantity = %d, want %d", got.Entry.Quantity, MaxQuantity)
	}
}

func TestARetriedAddWithTheSameKeyReturnsTheFirstAnswerAndAddsNothing(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	idem := &Idempotency{Key: "k1", RequestHash: []byte("h1")}

	first, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 2), idem)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 2), idem)
	if err != nil {
		t.Fatal(err)
	}

	if !retry.Replayed || !retry.Created || retry.Entry != first.Entry {
		t.Errorf("retry = %+v, want the first answer (%+v) marked as replayed", retry, first)
	}
	list, _, _ := r.List(ctx(), user, "", 10, 0)
	if len(list) != 1 || list[0].Quantity != 2 {
		t.Errorf("collection = %+v, want one entry of 2 (the retry added nothing)", list)
	}
}

func TestTheSameKeyForADifferentRequestIsRefused(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	if _, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 1), &Idempotency{Key: "k1", RequestHash: []byte("h1")}); err != nil {
		t.Fatal(err)
	}

	_, err := r.Add(ctx(), user, entry(gamma, "mint", "FR", 1), &Idempotency{Key: "k1", RequestHash: []byte("h2")})

	if !errors.Is(err, ErrKeyReused) {
		t.Errorf("err = %v, want ErrKeyReused", err)
	}
}

func TestAnotherKeyAddsAgainAndAnotherUserCanUseTheSameKey(t *testing.T) {
	r, tx := newRepo(t)
	ann, bob := newUser(t, tx, "ann"), newUser(t, tx, "bob")
	same := func(key string) *Idempotency { return &Idempotency{Key: key, RequestHash: []byte("h")} }

	_, _ = r.Add(ctx(), ann, entry(alpha, "mint", "FR", 1), same("k1"))
	second, _ := r.Add(ctx(), ann, entry(alpha, "mint", "FR", 1), same("k2"))
	forBob, err := r.Add(ctx(), bob, entry(alpha, "mint", "FR", 1), same("k1"))

	if second.Replayed || second.Entry.Quantity != 2 {
		t.Errorf("a new key must add: %+v", second)
	}
	if err != nil || forBob.Replayed || forBob.Entry.Quantity != 1 {
		t.Errorf("another user's key is another key: %+v, %v", forBob, err)
	}
}

func TestAnExpiredKeyIsForgottenAndTheRequestRunsAgain(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	idem := &Idempotency{Key: "k1", RequestHash: []byte("h")}
	mustAddKeyed(t, r, user, idem)
	mustExec(t, tx, `UPDATE idempotency_keys SET created_at = now() - interval '25 hours'`)

	again := mustAddKeyed(t, r, user, idem)

	if again.Replayed || again.Entry.Quantity != 2 {
		t.Errorf("after the delay the request must run again: %+v", again)
	}
}

func mustAddKeyed(t *testing.T, r *Repository, user string, idem *Idempotency) AddResult {
	t.Helper()
	res, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 1), idem)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
