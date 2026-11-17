package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

type okDB struct{}

func (okDB) Ping(context.Context) error { return nil }

type fakeReader struct {
	facets    catalog.Facets
	blocks    []catalog.BlockRow
	sets      []catalog.SetRow
	set       catalog.SetRow
	cards     []catalog.CardRow
	detail    catalog.CardDetailRow
	foil      catalog.FoilRow
	points    []catalog.PricePointRow
	hasMore   bool
	snapSets  []catalog.SnapshotSet
	snapCards []catalog.SnapshotCard
	snapCalls int
	err       error
	notFound  bool

	setFilter  catalog.SetFilter
	cardFilter catalog.CardFilter
	from, to   *string
}

func (f *fakeReader) result() error {
	if f.err != nil {
		return f.err
	}
	if f.notFound {
		return catalog.ErrNotFound
	}
	return nil
}

func (f *fakeReader) Blocks(context.Context) ([]catalog.BlockRow, error) { return f.blocks, f.result() }

func (f *fakeReader) Sets(_ context.Context, flt catalog.SetFilter) ([]catalog.SetRow, error) {
	f.setFilter = flt
	return f.sets, f.result()
}

func (f *fakeReader) Set(context.Context, string) (catalog.SetRow, error) { return f.set, f.result() }

func (f *fakeReader) SetCards(context.Context, string) ([]catalog.CardRow, error) {
	return f.cards, f.result()
}

func (f *fakeReader) Card(context.Context, string) (catalog.CardDetailRow, error) {
	return f.detail, f.result()
}

func (f *fakeReader) Foil(context.Context, string) (catalog.FoilRow, error) {
	return f.foil, f.result()
}

func (f *fakeReader) PriceHistory(_ context.Context, _ string, from, to *string) ([]catalog.PricePointRow, error) {
	f.from, f.to = from, to
	return f.points, f.result()
}

func (f *fakeReader) Snapshot(context.Context) ([]catalog.SnapshotSet, []catalog.SnapshotCard, error) {
	f.snapCalls++
	return f.snapSets, f.snapCards, f.result()
}

func (f *fakeReader) Facets(context.Context) (catalog.Facets, error) { return f.facets, f.result() }

func (f *fakeReader) SearchCards(_ context.Context, flt catalog.CardFilter) ([]catalog.CardRow, bool, error) {
	f.cardFilter = flt
	return f.cards, f.hasMore, f.result()
}

func newHandlerEngine(r Reader, imagesBase string) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return mount(httpserver.New(logger), NewHandler(r, NewImageURLs(imagesBase), logger))
}

func mount(engine *gin.Engine, h *Handler) *gin.Engine {
	g := engine.Group("/v1", OnlyFrench)
	g.GET("/blocks", h.Blocks)
	g.GET("/sets", h.Sets)
	g.GET("/sets/:setId", h.Set)
	g.GET("/sets/:setId/cards", h.SetCards)
	g.GET("/cards", h.SearchCards)
	g.GET("/cards/facets", h.Facets)
	g.GET("/cards/:cardId", h.Card)
	g.GET("/cards/:cardId/foil", h.Foil)
	g.GET("/cards/:cardId/price-history", h.PriceHistory)
	g.GET("/catalog/snapshot", h.Snapshot)
	return engine
}

func get(t *testing.T, h http.Handler, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, rec.Body.String())
	}
	return m
}

func expectProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if got := decode(t, rec)["code"]; got != code {
		t.Errorf("code = %v, want %s", got, code)
	}
}
