package collection

import (
	"testing"
	"time"
)

func TestListPagesNewestFirstAndFiltersBySet(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	for i, card := range []string{alpha, beta, gamma, delta} {
		mustAdd(t, r, user, entry(card, "mint", "FR", 1))

		mustExec(t, tx, `UPDATE collection_entries SET added_at = $1 WHERE user_id = $2 AND card_id = $3`,
			time.Now().Add(-time.Duration(4-i)*time.Hour), user, card)
	}

	first, more, err := r.List(ctx(), user, "", 3, 0)
	if err != nil || !more || len(first) != 3 || first[0].CardID != delta || first[2].CardID != beta {
		t.Fatalf("page 1 = %+v, more %v, %v; want delta, gamma, beta and more", first, more, err)
	}
	last, more, err := r.List(ctx(), user, "", 3, 3)
	if err != nil || more || len(last) != 1 || last[0].CardID != alpha {
		t.Fatalf("page 2 = %+v, more %v, %v; want alpha only", last, more, err)
	}
	inSet, more, _ := r.List(ctx(), user, "zzt1", 10, 0)
	if more || len(inSet) != 3 {
		t.Errorf("set zzt1 = %+v, want 3 entries", inSet)
	}
}
