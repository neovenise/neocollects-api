package collection

import (
	"errors"
	"testing"
)

func TestOneUserCannotSeeChangeOrDeleteAnotherUsersEntries(t *testing.T) {
	r, tx := newRepo(t)
	ann, bob := newUser(t, tx, "ann"), newUser(t, tx, "bob")
	annsEntry := mustAdd(t, r, ann, entry(alpha, "mint", "FR", 1)).Entry
	qty := 50

	if list, _, _ := r.List(ctx(), bob, "", 10, 0); len(list) != 0 {
		t.Errorf("bob sees %+v", list)
	}
	if _, err := r.Update(ctx(), bob, annsEntry.ID, Patch{Quantity: &qty}); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("bob updates: err = %v, want ErrEntryNotFound", err)
	}
	if err := r.Delete(ctx(), bob, annsEntry.ID); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("bob deletes: err = %v, want ErrEntryNotFound", err)
	}
	if s, _ := r.Summary(ctx(), bob); s.TotalQuantity != 0 || len(s.Sets) != 0 {
		t.Errorf("bob's summary = %+v, want empty", s)
	}
	list, _, _ := r.List(ctx(), ann, "", 10, 0)
	if len(list) != 1 || list[0].Quantity != 1 {
		t.Errorf("ann's entry was touched: %+v", list)
	}
}

func TestUpdatingTheQuantity(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	e := mustAdd(t, r, user, entry(alpha, "mint", "FR", 1)).Entry
	qty := 7

	got, err := r.Update(ctx(), user, e.ID, Patch{Quantity: &qty})

	if err != nil || got.ID != e.ID || got.Quantity != 7 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestChangingTheConditionMovesTheEntryWhenNothingIsInTheWay(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	e := mustAdd(t, r, user, entry(alpha, "mint", "FR", 2)).Entry
	played := "played"
	en := "EN"

	got, err := r.Update(ctx(), user, e.ID, Patch{Condition: &played, Language: &en})

	if err != nil || got.ID != e.ID || got.Condition != "played" || got.Language != "EN" || got.Quantity != 2 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestChangingTheConditionToOneThatExistsMergesTheTwoEntries(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mint := mustAdd(t, r, user, entry(alpha, "mint", "FR", 2)).Entry
	played := mustAdd(t, r, user, entry(alpha, "played", "FR", 3)).Entry
	cond := "mint"

	got, err := r.Update(ctx(), user, played.ID, Patch{Condition: &cond})

	if err != nil {
		t.Fatal(err)
	}
	if got.ID != mint.ID || got.Quantity != 5 {
		t.Errorf("merged = %+v, want the existing mint entry with 5", got)
	}
	list, _, _ := r.List(ctx(), user, "", 10, 0)
	if len(list) != 1 {
		t.Errorf("collection = %+v, want a single entry (the merged one is gone)", list)
	}
}

func TestAMergeOverTheMaximumChangesNothing(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustAdd(t, r, user, entry(alpha, "mint", "FR", MaxQuantity))
	played := mustAdd(t, r, user, entry(alpha, "played", "FR", 1)).Entry
	cond := "mint"

	_, err := r.Update(ctx(), user, played.ID, Patch{Condition: &cond})

	if !errors.Is(err, ErrQuantityTooLarge) {
		t.Fatalf("err = %v, want ErrQuantityTooLarge", err)
	}
	if list, _, _ := r.List(ctx(), user, "", 10, 0); len(list) != 2 {
		t.Errorf("collection = %+v, want both entries still there", list)
	}
}

func TestDeletingAnEntry(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	e := mustAdd(t, r, user, entry(alpha, "mint", "FR", 1)).Entry

	if err := r.Delete(ctx(), user, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx(), user, e.ID); !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("second delete: err = %v, want ErrEntryNotFound", err)
	}
}
