package auth

import (
	"context"
	"os"
	"testing"
	"time"

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

const invite = "the-invitation-code"

func newService(t *testing.T) (*Service, *time.Time, pgx.Tx) {
	t.Helper()
	if testPool == nil {
		t.Skip("no test database: set DATABASE_URL")
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	s, err := NewService(NewRepository(tx), testHasher, DefaultConfig(invite))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	s.byUsername.now, s.byAddress.now = s.now, s.now
	return s, &now, tx
}

func problem(t *testing.T, err error, code string) {
	t.Helper()
	p, ok := err.(*Problem)
	if !ok || p.Code != code {
		t.Errorf("err = %v, want a problem %q", err, code)
	}
}

func register(t *testing.T, s *Service, username, password string) Session {
	t.Helper()
	session, err := s.Register(context.Background(), "1.1.1.1", username, password, invite)
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	return session
}
