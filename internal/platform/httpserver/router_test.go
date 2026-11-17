package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func newTestEngine(db Pinger) http.Handler {
	engine := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	engine.GET("/healthz", Health(db))
	return engine
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem {
	t.Helper()
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not a problem: %v", err)
	}
	return p
}

func TestHealthzOK(t *testing.T) {
	rec := do(t, newTestEngine(fakeDB{}), http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHealthzWhenDatabaseIsDown(t *testing.T) {
	rec := do(t, newTestEngine(fakeDB{err: errors.New("down")}), http.MethodGet, "/healthz")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "database_unavailable" || p.Status != 503 {
		t.Errorf("unexpected problem: %+v", p)
	}
}

func TestUnknownRouteIsAProblem(t *testing.T) {
	rec := do(t, newTestEngine(fakeDB{}), http.MethodGet, "/nope")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "route_not_found" {
		t.Errorf("code = %q", p.Code)
	}
}

func TestWrongMethodIsAProblem(t *testing.T) {
	rec := do(t, newTestEngine(fakeDB{}), http.MethodPost, "/healthz")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "method_not_allowed" {
		t.Errorf("code = %q", p.Code)
	}
}

func TestPanicBecomesAProblem(t *testing.T) {
	engine := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	engine.GET("/boom", func(c *gin.Context) { panic("boom") })

	rec := do(t, engine, http.MethodGet, "/boom")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Code != "internal_error" || p.Detail != "" {
		t.Errorf("the panic message must not leak: %+v", p)
	}
}
