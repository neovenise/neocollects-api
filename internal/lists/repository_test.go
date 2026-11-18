package lists

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neovenise/neocollects-api/internal/platform/database"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	if url := database.TestURL(); url != "" {
		if err := database.Migrate(url, "../../migrations"); err != nil {
			println("migrations failed:", err.Error())
			os.Exit(1)
		}
		pool, err := database.NewPool(context.Background(), url)
		if err != nil {
			println("database unreachable:", err.Error())
			os.Exit(1)
		}
		testPool = pool
	}
	code := m.Run()
	if testPool != nil {
		testPool.Close()
	}
	os.Exit(code)
}

const (
	alpha, beta, gamma, digital = "zzl1-001", "zzl1-002", "zzl1-003", "zzld-001"
)

func newTx(t *testing.T) pgx.Tx {
	t.Helper()
	if testPool == nil {
		t.Skip("no test database: set DATABASE_URL")
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	mustExec(t, tx, `INSERT INTO blocks (id, name, position, is_digital) VALUES ('zzl', 'Test', 0, false), ('zzld', 'Test digital', 0, true)`)
	mustExec(t, tx, `INSERT INTO sets (id, name, block_id, official_count, total_count) VALUES
		('zzl1', 'Test one', 'zzl', 3, 3), ('zzld', 'Test digital', 'zzld', 1, 1)`)
	mustExec(t, tx, `INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity) VALUES
		('zzl1-001', 'zzl1', '001', 1, 'Alpha', 'alpha', ''), ('zzl1-002', 'zzl1', '002', 2, 'Beta', 'beta', ''),
		('zzl1-003', 'zzl1', '003', 3, 'Gamma', 'gamma', ''), ('zzld-001', 'zzld', '001', 1, 'Digital', 'digital', '')`)
	return tx
}

func mustExec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func newUser(t *testing.T, tx pgx.Tx, name string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(), `INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id::text`, name).Scan(&id); err != nil {
		t.Fatalf("new user: %v", err)
	}
	return id
}

func newRepo(t *testing.T) (*Repository, pgx.Tx) {
	t.Helper()
	tx := newTx(t)
	return NewRepository(tx), tx
}

func ctx() context.Context { return context.Background() }

func mustLists(t *testing.T, r *Repository, user string, withIDs bool) []List {
	t.Helper()
	l, err := r.Lists(ctx(), user, withIDs)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func cardIDs(l List) []string {
	ids := []string{}
	for _, c := range l.Cards {
		ids = append(ids, c.CardID)
	}
	return ids
}

func byKind(lists []List, kind string) List {
	for _, l := range lists {
		if l.Kind == kind {
			return l
		}
	}
	return List{}
}

func TestOneUserCannotSeeOrChangeAnotherUsersLists(t *testing.T) {
	r, tx := newRepo(t)
	ann, bob := newUser(t, tx, "ann"), newUser(t, tx, "bob")
	custom, _ := r.Create(ctx(), ann, "Secrète")
	_ = r.PutCard(ctx(), ann, custom.ID, alpha, nil)

	if _, err := r.Rename(ctx(), bob, custom.ID, "Piratée"); !errors.Is(err, ErrListNotFound) {
		t.Errorf("rename: err = %v, want ErrListNotFound", err)
	}
	if err := r.Delete(ctx(), bob, custom.ID); !errors.Is(err, ErrListNotFound) {
		t.Errorf("delete: err = %v, want ErrListNotFound", err)
	}
	if err := r.PutCard(ctx(), bob, custom.ID, beta, nil); !errors.Is(err, ErrListNotFound) {
		t.Errorf("add: err = %v, want ErrListNotFound", err)
	}
	if err := r.RemoveCard(ctx(), bob, custom.ID, alpha); !errors.Is(err, ErrListNotFound) {
		t.Errorf("remove: err = %v, want ErrListNotFound", err)
	}
	if lists := mustLists(t, r, bob, true); len(lists) != 3 {
		t.Errorf("bob sees %+v, want only his own three", lists)
	}
	if got := byKind(mustLists(t, r, ann, true), "custom"); got.Name != "Secrète" || got.CardCount != 1 {
		t.Errorf("ann's list was touched: %+v", got)
	}
}

func TestDeletingAnAccountRemovesItsLists(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	favorites := byKind(mustLists(t, r, user, false), "favorites")
	_ = r.PutCard(ctx(), user, favorites.ID, alpha, nil)

	mustExec(t, tx, `DELETE FROM users WHERE id = $1`, user)

	var lists, cards int
	_ = tx.QueryRow(ctx(), `SELECT (SELECT count(*) FROM lists WHERE user_id = $1), (SELECT count(*) FROM list_cards WHERE list_id = $2)`, user, favorites.ID).Scan(&lists, &cards)
	if lists != 0 || cards != 0 {
		t.Errorf("left %d lists and %d list cards", lists, cards)
	}
}
