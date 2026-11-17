package collection

import "testing"

func TestSummaryValuesWithTheCurrentPricesAndLeavesOutAPriceThatIsNotTheCardsOwn(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustAdd(t, r, user, entry(alpha, "mint", "FR", 2))
	mustAdd(t, r, user, entry(alpha, "played", "FR", 1))
	mustAdd(t, r, user, entry(beta, "mint", "FR", 1))
	mustAdd(t, r, user, entry(gamma, "mint", "FR", 1))
	mustAdd(t, r, user, entry(delta, "mint", "FR", 4))

	s, err := r.Summary(ctx(), user)

	if err != nil {
		t.Fatal(err)
	}
	if s.TotalQuantity != 9 || s.DistinctCards != 4 || s.ValueCents != 4000 || s.UnpricedCards != 2 {
		t.Errorf("summary = %+v, want 9 copies, 4 cards, 4000 c, 2 unpriced", s)
	}
	if len(s.Sets) != 2 || s.Sets[0] != (SetCount{"zzt1", 3}) || s.Sets[1] != (SetCount{"zzt2", 1}) {
		t.Errorf("sets = %+v, want zzt1:3 and zzt2:1 (distinct cards, not copies)", s.Sets)
	}
}

func TestSummaryFollowsAPriceChangeBecauseTheValueIsNeverStored(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustAdd(t, r, user, entry(delta, "mint", "FR", 2))
	mustExec(t, tx, `UPDATE card_prices SET cents = 300 WHERE card_id = $1`, delta)

	s, _ := r.Summary(ctx(), user)

	if s.ValueCents != 600 {
		t.Errorf("value = %d, want 600", s.ValueCents)
	}
}

func TestTheValueOfABigCollectionDoesNotOverflow(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustExec(t, tx, `UPDATE card_prices SET cents = 2000000 WHERE card_id = $1`, delta)
	mustAdd(t, r, user, entry(delta, "mint", "FR", MaxQuantity))

	s, err := r.Summary(ctx(), user)

	if err != nil || s.ValueCents != int64(MaxQuantity)*2000000 {
		t.Errorf("value = %d, err %v, want %d", s.ValueCents, err, int64(MaxQuantity)*2000000)
	}
}

func TestAnEmptyCollectionHasAnEmptySummaryAndAnEmptyListNotNulls(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	s, err := r.Summary(ctx(), user)
	list, more, _ := r.List(ctx(), user, "", 10, 0)

	if err != nil || s.Sets == nil || s.TotalQuantity != 0 || s.ValueCents != 0 {
		t.Errorf("summary = %+v, %v", s, err)
	}
	if list == nil || more {
		t.Errorf("list = %#v, more %v, want an empty non-nil slice", list, more)
	}
}
