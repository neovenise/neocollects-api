package lists

import (
	"errors"
	"strings"
	"testing"
)

func TestAddingCardsKeepsTheOrderOfAdditionAndTheFirstThreeAsPreview(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	wishlist := byKind(mustLists(t, r, user, false), "wishlist")

	for _, card := range []string{gamma, alpha, beta} {
		if err := r.PutCard(ctx(), user, wishlist.ID, card, nil); err != nil {
			t.Fatal(err)
		}
	}

	got := byKind(mustLists(t, r, user, true), "wishlist")
	if got.CardCount != 3 || strings.Join(cardIDs(got), ",") != gamma+","+alpha+","+beta || strings.Join(got.PreviewCardIDs, ",") != gamma+","+alpha+","+beta {
		t.Errorf("list = %+v, want gamma, alpha, beta in that order", got)
	}
	if without := byKind(mustLists(t, r, user, false), "wishlist"); without.Cards != nil || without.CardCount != 3 {
		t.Errorf("without include the cards must be absent: %+v", without)
	}
}

func TestThePreviewIsCutToThreeCardsAndANewCardGoesLast(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")
	mustExec(t, tx, `INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity) VALUES ('zzl1-004', 'zzl1', '004', 4, 'Delta', 'delta', '')`)
	for _, card := range []string{alpha, beta, gamma, "zzl1-004"} {
		if err := r.PutCard(ctx(), user, favorites.ID, card, nil); err != nil {
			t.Fatal(err)
		}
	}

	got := byKind(mustLists(t, r, user, true), "favorites")

	if len(got.PreviewCardIDs) != 3 || got.CardCount != 4 || got.Cards[3].CardID != "zzl1-004" {
		t.Errorf("list = %+v", got)
	}
}

func TestAddingTheSameCardTwiceChangesNothing(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")

	for i := 0; i < 3; i++ {
		if err := r.PutCard(ctx(), user, favorites.ID, alpha, nil); err != nil {
			t.Fatalf("add #%d: %v", i, err)
		}
	}

	if got := byKind(mustLists(t, r, user, true), "favorites"); got.CardCount != 1 {
		t.Errorf("list = %+v, want one card", got)
	}
}

func TestAddingAnUnknownOrDigitalOnlyCardIsRefused(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")

	for _, card := range []string{"nope-1", digital} {
		if err := r.PutCard(ctx(), user, favorites.ID, card, nil); !errors.Is(err, ErrCardNotFound) {
			t.Errorf("add %q: err = %v, want ErrCardNotFound", card, err)
		}
	}
}

func TestAListHasALimitButAnExistingCardIsStillAccepted(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")
	if err := r.PutCard(ctx(), user, favorites.ID, alpha, nil); err != nil {
		t.Fatal(err)
	}

	mustExec(t, tx, `INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity)
		SELECT 'zzl1-f' || n, 'zzl1', 'f' || n, NULL, 'F' || n, 'f' || n, '' FROM generate_series(1, $1) n`, MaxCardsPerList-1)
	mustExec(t, tx, `INSERT INTO list_cards (list_id, card_id, position) SELECT $1, 'zzl1-f' || n, n + 1 FROM generate_series(1, $2) n`, favorites.ID, MaxCardsPerList-1)

	if err := r.PutCard(ctx(), user, favorites.ID, beta, nil); !errors.Is(err, ErrListFull) {
		t.Errorf("a new card in a full list: err = %v, want ErrListFull", err)
	}
	if err := r.PutCard(ctx(), user, favorites.ID, alpha, nil); err != nil {
		t.Errorf("a card already there is not 'one more': %v", err)
	}
}

func TestRemovingACardAndRemovingItAgain(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")
	_ = r.PutCard(ctx(), user, favorites.ID, alpha, nil)
	_ = r.PutCard(ctx(), user, favorites.ID, beta, nil)

	for i := 0; i < 2; i++ {
		if err := r.RemoveCard(ctx(), user, favorites.ID, alpha); err != nil {
			t.Fatalf("remove #%d: %v", i, err)
		}
	}

	if got := byKind(mustLists(t, r, user, true), "favorites"); strings.Join(cardIDs(got), ",") != beta {
		t.Errorf("list = %+v, want only beta", got)
	}
}

func TestAFullListStillAcceptsANewQuantityForACardItHolds(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")
	_ = r.PutCard(ctx(), user, favorites.ID, alpha, nil)
	mustExec(t, tx, `INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity)
		SELECT 'zzl1-f' || n, 'zzl1', 'f' || n, NULL, 'F' || n, 'f' || n, '' FROM generate_series(1, $1) n`, MaxCardsPerList-1)
	mustExec(t, tx, `INSERT INTO list_cards (list_id, card_id, position) SELECT $1, 'zzl1-f' || n, n + 1 FROM generate_series(1, $2) n`, favorites.ID, MaxCardsPerList-1)

	two := 2
	if err := r.PutCard(ctx(), user, favorites.ID, alpha, &two); err != nil {
		t.Errorf("changing a quantity is not adding a card: %v", err)
	}
	if err := r.PutCard(ctx(), user, favorites.ID, beta, &two); !errors.Is(err, ErrListFull) {
		t.Errorf("a new card with a quantity in a full list: err = %v, want ErrListFull", err)
	}
}
