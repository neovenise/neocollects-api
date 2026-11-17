package app

import (
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/auth"
	catalogapi "github.com/neovenise/neocollects-api/internal/catalog/api"
	"github.com/neovenise/neocollects-api/internal/collection"
	"github.com/neovenise/neocollects-api/internal/lists"
)

func TestTheRouteTableIsTheAPIOfDocsAPIMD(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := gin.New()
	Routes(engine, Handlers{
		Health:     func(*gin.Context) {},
		Catalog:    catalogapi.NewHandler(nil, catalogapi.NewImageURLs(""), logger),
		Auth:       auth.NewHandler(nil, logger),
		Collection: collection.NewHandler(nil, logger),
		Lists:      lists.NewHandler(nil, logger),
	})

	var got []string
	for _, r := range engine.Routes() {
		got = append(got, r.Method+" "+r.Path)
	}
	sort.Strings(got)
	want := []string{
		"DELETE /v1/me", "DELETE /v1/me/collection/entries/:entryID", "DELETE /v1/me/lists/:listID",
		"DELETE /v1/me/lists/:listID/cards/:cardID",
		"GET /healthz", "GET /v1/blocks", "GET /v1/cards", "GET /v1/cards/:cardId", "GET /v1/cards/:cardId/foil",
		"GET /v1/cards/:cardId/price-history", "GET /v1/cards/facets", "GET /v1/catalog/snapshot", "GET /v1/me",
		"GET /v1/me/collection/entries", "GET /v1/me/collection/summary", "GET /v1/me/lists", "GET /v1/sets",
		"GET /v1/sets/:setId", "GET /v1/sets/:setId/cards",
		"PATCH /v1/me/collection/entries/:entryID", "PATCH /v1/me/lists/:listID",
		"POST /v1/auth/login", "POST /v1/auth/logout", "POST /v1/auth/refresh", "POST /v1/auth/register",
		"POST /v1/me/collection/entries", "POST /v1/me/lists",
		"PUT /v1/me/lists/:listID/cards/:cardID", "PUT /v1/me/password",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("routes:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
