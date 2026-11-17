package lists

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neovenise/neocollects-api/internal/auth"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

type okDB struct{}

func (okDB) Ping(context.Context) error { return nil }

type api struct {
	t        *testing.T
	h        http.Handler
	ann, bob string
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

	me := engine.Group("/v1/me/lists", authRoutes.Required())
	me.GET("", h.List)
	me.POST("", h.Create)
	me.PATCH("/:listID", h.Rename)
	me.DELETE("/:listID", h.Remove)
	me.PUT("/:listID/cards/:cardID", h.PutCard)
	me.DELETE("/:listID/cards/:cardID", h.RemoveCard)
	a := &api{t: t, h: engine}
	ann, err := accounts.Register(context.Background(), "10.0.0.1", "ann-http", "a long enough password", "invite")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := accounts.Register(context.Background(), "10.0.0.1", "bob-http", "a long enough password", "invite")
	if err != nil {
		t.Fatal(err)
	}
	a.ann, a.bob = ann.AccessToken, bob.AccessToken
	return a
}

func (a *api) call(token, method, target, body string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func object(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON (%d): %q", rec.Code, rec.Body.String())
	}
	return out
}

func (a *api) items(token, query string) []map[string]any {
	a.t.Helper()
	rec := a.call(token, "GET", "/v1/me/lists"+query, "")
	raw, _ := object(a.t, rec)["items"].([]any)
	out := make([]map[string]any, len(raw))
	for i, item := range raw {
		out[i], _ = item.(map[string]any)
	}
	return out
}

func (a *api) listOfKind(token, kind string) map[string]any {
	a.t.Helper()
	for _, l := range a.items(token, "?include=cards") {
		if l["kind"] == kind {
			return l
		}
	}
	a.t.Fatalf("no %s list", kind)
	return nil
}
