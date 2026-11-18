package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/database"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	if url := database.TestURL(); url != "" {
		if err := database.Migrate(url, "../../../migrations"); err != nil {
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

func newTx(t *testing.T) (*Repository, pgx.Tx) {
	t.Helper()
	if testPool == nil {
		t.Skip("no test database: set DATABASE_URL")
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return NewRepository(tx), tx
}

func seed(t *testing.T, repo *Repository) {
	t.Helper()
	ctx := context.Background()
	if err := repo.UpsertBlock(ctx, catalog.Block{ID: "sv", Name: "Écarlate et Violet", Position: 1}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertSet(ctx, catalog.Set{ID: "sv03", Code: "OBF", Name: "Flammes Obsidiennes", BlockID: "sv", OfficialCount: 197, TotalCount: 230}); err != nil {
		t.Fatal(err)
	}
}

type historyRow struct {
	Day   string
	Cents int
}

func history(t *testing.T, tx pgx.Tx, cardID string) []historyRow {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT day::text, cents FROM card_price_history WHERE card_id = $1 ORDER BY day`, cardID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []historyRow
	for rows.Next() {
		var r historyRow
		if err := rows.Scan(&r.Day, &r.Cents); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func price(cardID string, cents int, day string) catalog.Price {
	t, _ := time.Parse(time.DateOnly, day)
	return catalog.Price{CardID: cardID, Cents: cents, Currency: "EUR", UpdatedAt: t.Add(22 * time.Hour)}
}

// setImageKeys records the pictures that neocollects-images copied for a card.
func setImageKeys(t *testing.T, tx pgx.Tx, cardID, imageKey, thumbKey, hash string) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `UPDATE cards SET image_key = $2, thumb_key = $3, image_source_hash = $4 WHERE id = $1`, cardID, imageKey, thumbKey, hash); err != nil {
		t.Fatal(err)
	}
}
