package lists

import (
	"errors"
	"testing"
)

func TestANewAccountHasTheThreeDefaultListsInOrderAndEmpty(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	lists := mustLists(t, r, user, false)

	if len(lists) != 3 || lists[0].Kind != "favorites" || lists[1].Kind != "trade" || lists[2].Kind != "wishlist" {
		t.Fatalf("lists = %+v, want favorites, trade, wishlist", lists)
	}
	if lists[0].Name != "Favoris" || lists[1].Name != "À échanger" || lists[2].Name != "Liste de souhaits" {
		t.Errorf("names = %q %q %q", lists[0].Name, lists[1].Name, lists[2].Name)
	}
	for _, l := range lists {
		if l.CardCount != 0 || l.PreviewCardIDs == nil || len(l.PreviewCardIDs) != 0 || l.Cards != nil {
			t.Errorf("%s = %+v, want an empty list with a non-nil empty preview and no cards", l.Kind, l)
		}
	}
}

func TestADefaultListCannotBeDuplicated(t *testing.T) {
	_, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	tag, err := tx.Exec(ctx(), `INSERT INTO lists (user_id, kind, name) VALUES ($1, 'favorites', 'Encore') ON CONFLICT DO NOTHING`, user)

	if err != nil || tag.RowsAffected() != 0 {
		t.Errorf("a second favorites list was accepted (%v, %v)", tag, err)
	}
}

func TestCreatingARenamingAndDeletingACustomList(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")

	created, err := r.Create(ctx(), user, "Mes holos")
	if err != nil || created.Kind != "custom" || created.Name != "Mes holos" || created.CardCount != 0 {
		t.Fatalf("created = %+v, %v", created, err)
	}
	if lists := mustLists(t, r, user, false); len(lists) != 4 || lists[3].ID != created.ID {
		t.Fatalf("lists = %+v, want the custom one last", lists)
	}
	renamed, err := r.Rename(ctx(), user, created.ID, "Mes secrètes")
	if err != nil || renamed.Name != "Mes secrètes" || renamed.ID != created.ID {
		t.Errorf("renamed = %+v, %v", renamed, err)
	}
	if err := r.Delete(ctx(), user, created.ID); err != nil {
		t.Fatal(err)
	}
	if lists := mustLists(t, r, user, false); len(lists) != 3 {
		t.Errorf("lists = %+v, want the three default ones", lists)
	}
}

func TestTheDefaultListsCannotBeRenamedOrDeleted(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")

	if _, err := r.Rename(ctx(), user, favorites.ID, "Autre"); !errors.Is(err, ErrDefaultList) {
		t.Errorf("rename: err = %v, want ErrDefaultList", err)
	}
	if err := r.Delete(ctx(), user, favorites.ID); !errors.Is(err, ErrDefaultList) {
		t.Errorf("delete: err = %v, want ErrDefaultList", err)
	}
	if got := byKind(mustLists(t, r, user, false), "favorites"); got.Name != "Favoris" {
		t.Errorf("favorites = %+v, want it untouched", got)
	}
}

func TestThereIsALimitToTheCustomLists(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	mustExec(t, tx, `INSERT INTO lists (user_id, kind, name) SELECT $1, 'custom', 'l' || n FROM generate_series(1, $2) n`, user, MaxCustomLists)

	_, err := r.Create(ctx(), user, "Une de trop")

	if !errors.Is(err, ErrTooManyLists) {
		t.Errorf("err = %v, want ErrTooManyLists", err)
	}
	if _, err := r.Create(ctx(), newUser(t, tx, "bob"), "Mine"); err != nil {
		t.Errorf("another user is not limited by ann's lists: %v", err)
	}
}

func TestDeletingACustomListRemovesItsCardsButNotTheCatalog(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	custom, _ := r.Create(ctx(), user, "À moi")
	_ = r.PutCard(ctx(), user, custom.ID, alpha, nil)

	if err := r.Delete(ctx(), user, custom.ID); err != nil {
		t.Fatal(err)
	}

	var inLists, inCatalog int
	_ = tx.QueryRow(ctx(), `SELECT (SELECT count(*) FROM list_cards WHERE list_id = $1), (SELECT count(*) FROM cards WHERE id = $2)`, custom.ID, alpha).Scan(&inLists, &inCatalog)
	if inLists != 0 || inCatalog != 1 {
		t.Errorf("list_cards = %d, cards = %d, want 0 and 1", inLists, inCatalog)
	}
}
