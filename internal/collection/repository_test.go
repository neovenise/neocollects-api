package collection

import (
	"context"
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
	alpha, beta, gamma, delta, digital = "zzt1-001", "zzt1-002", "zzt1-003", "zzt2-001", "zztd-001"
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
	seedCatalog(t, tx)
	return tx
}

func mustExec(t *testing.T, tx pgx.Tx, sql string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func seedCatalog(t *testing.T, tx pgx.Tx) {
	t.Helper()
	mustExec(t, tx, `INSERT INTO blocks (id, name, position, is_digital) VALUES ('zzt', 'Test', 0, false), ('zztd', 'Test digital', 0, true)`)
	mustExec(t, tx, `INSERT INTO sets (id, name, block_id, official_count, total_count) VALUES
		('zzt1', 'Test one', 'zzt', 3, 3), ('zzt2', 'Test two', 'zzt', 1, 1), ('zztd', 'Test digital', 'zztd', 1, 1)`)
	mustExec(t, tx, `INSERT INTO cards (id, set_id, local_id, local_number, name, name_search, rarity) VALUES
		('zzt1-001', 'zzt1', '001', 1, 'Alpha', 'alpha', 'Rare'),
		('zzt1-002', 'zzt1', '002', 2, 'Beta',  'beta',  'Commune'),
		('zzt1-003', 'zzt1', '003', 3, 'Gamma', 'gamma', 'Commune'),
		('zzt2-001', 'zzt2', '001', 1, 'Delta', 'delta', 'Commune'),
		('zztd-001', 'zztd', '001', 1, 'Digital', 'digital', 'Commune')`)
	mustExec(t, tx, `INSERT INTO card_prices (card_id, cents, currency, updated_at, product_id) VALUES
		('zzt1-001', 1000, 'EUR', now(), 77), ('zzt1-002', 1000, 'EUR', now(), 77), ('zzt2-001', 250, 'EUR', now(), 78)`)
}

func newUser(t *testing.T, tx pgx.Tx, name string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(),
		`INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id::text`, name).Scan(&id); err != nil {
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

func entry(card, condition, language string, quantity int) NewEntry {
	return NewEntry{CardID: card, Condition: condition, Language: language, Quantity: quantity}
}

func mustAdd(t *testing.T, r *Repository, user string, in NewEntry) AddResult {
	t.Helper()
	res, err := r.Add(ctx(), user, in, nil)
	if err != nil {
		t.Fatalf("add %+v: %v", in, err)
	}
	return res
}

func TestDeletingAnAccountRemovesItsCollectionAndKeys(t *testing.T) {
	r, tx := newRepo(t)
	user := newUser(t, tx, "ann")
	if _, err := r.Add(ctx(), user, entry(alpha, "mint", "FR", 1), &Idempotency{Key: "k", RequestHash: []byte("h")}); err != nil {
		t.Fatal(err)
	}

	mustExec(t, tx, `DELETE FROM users WHERE id = $1`, user)

	var entries, keys int
	_ = tx.QueryRow(ctx(), `SELECT (SELECT count(*) FROM collection_entries WHERE user_id = $1), (SELECT count(*) FROM idempotency_keys WHERE user_id = $1)`, user).Scan(&entries, &keys)
	if entries != 0 || keys != 0 {
		t.Errorf("left %d entries and %d keys after the account was deleted", entries, keys)
	}
}
