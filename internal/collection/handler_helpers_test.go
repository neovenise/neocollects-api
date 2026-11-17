package collection

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/neovenise/neocollects-api/internal/auth"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

type okDB struct{}

func (okDB) Ping(context.Context) error { return nil }

type api struct {
	t          *testing.T
	h          http.Handler
	tx         pgx.Tx
	ann, bob   string
	annsUserID string
}

func newAPI(t *testing.T) *api {
	t.Helper()
	tx := newTx(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	accounts, err := auth.NewService(auth.NewRepository(tx), auth.Hasher{Memory: 64, Time: 1, Threads: 1}, auth.DefaultConfig("invite"))
	if err != nil {
		t.Fatal(err)
	}
	authRoutes := auth.NewHandler(accounts, logger)
	h := NewHandler(NewRepository(tx), logger)
	engine := httpserver.New(logger)

	me := engine.Group("/v1/me/collection", authRoutes.Required())
	me.GET("/entries", h.List)
	me.POST("/entries", h.Add)
	me.PATCH("/entries/:entryID", h.Update)
	me.DELETE("/entries/:entryID", h.Remove)
	me.GET("/summary", h.Summary)
	a := &api{t: t, tx: tx, h: engine}
	ann, err := accounts.Register(context.Background(), "10.0.0.1", "ann-http", "a long enough password", "invite")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := accounts.Register(context.Background(), "10.0.0.1", "bob-http", "a long enough password", "invite")
	if err != nil {
		t.Fatal(err)
	}
	a.ann, a.bob, a.annsUserID = ann.AccessToken, bob.AccessToken, ann.User.ID
	return a
}

func (a *api) call(token, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func decodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON (%d): %q", rec.Code, rec.Body.String())
	}
	return out
}

func body(card, condition, language string, quantity int) string {
	b, _ := json.Marshal(map[string]any{"cardId": card, "condition": condition, "language": language, "quantity": quantity})
	return string(b)
}
