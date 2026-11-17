package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestBlocksShapeAndCaching(t *testing.T) {
	h := newHandlerEngine(&fakeReader{blocks: []catalog.BlockRow{{ID: "sv", Name: "Écarlate et Violet"}}}, "")

	rec := get(t, h, "/v1/blocks")

	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != `{"items":[{"id":"sv","name":"Écarlate et Violet"}]}` {
		t.Errorf("body = %s", rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=300" || rec.Header().Get("ETag") == "" {
		t.Errorf("headers = %v", rec.Header())
	}

	again := get(t, h, "/v1/blocks", "If-None-Match", rec.Header().Get("ETag"))
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("a client with the current ETag must get 304 and no body: %d (%d bytes)", again.Code, again.Body.Len())
	}
	weak := get(t, h, "/v1/blocks", "If-None-Match", `W/`+rec.Header().Get("ETag")+`, "other"`)
	if weak.Code != http.StatusNotModified {
		t.Errorf("weak validators in a list must match: %d", weak.Code)
	}
	if stale := get(t, h, "/v1/blocks", "If-None-Match", `"stale"`); stale.Code != 200 {
		t.Errorf("a stale ETag must get the body: %d", stale.Code)
	}
}

func TestEmptyListsAreArraysNotNull(t *testing.T) {
	h := newHandlerEngine(&fakeReader{}, "")

	for _, path := range []string{"/v1/blocks", "/v1/sets", "/v1/sets/x/cards", "/v1/cards", "/v1/cards/x/price-history"} {
		if body := get(t, h, path).Body.String(); !strings.Contains(body, `"items":[]`) {
			t.Errorf("%s: %s", path, body)
		}
	}
}

func TestSetsFiltersAndShape(t *testing.T) {
	date := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	r := &fakeReader{sets: []catalog.SetRow{{ID: "30th", Code: "30C", Name: "30ᵉ Anniversaire", BlockID: "me", BlockName: "Méga-Évolution",
		OfficialCount: 128, TotalCount: 161, ReleaseDate: &date, LogoKey: str("sets/30th.logo.1234abcd.webp")}}}
	h := newHandlerEngine(r, "https://img.test")

	rec := get(t, h, "/v1/sets?blockId=me&officialCount=128")

	if r.setFilter.BlockID != "me" || r.setFilter.OfficialCount == nil || *r.setFilter.OfficialCount != 128 {
		t.Errorf("filter = %+v", r.setFilter)
	}
	item := decode(t, rec)["items"].([]any)[0].(map[string]any)
	if item["id"] != "30th" || item["code"] != "30C" || item["blockName"] != "Méga-Évolution" ||
		item["officialCount"] != float64(128) || item["releaseDate"] != "2026-09-16" || item["logoUrl"] != "https://img.test/sets/30th.logo.1234abcd.webp" {
		t.Errorf("item = %v", item)
	}
	if _, has := item["symbolUrl"]; has {
		t.Error("an unknown symbol must be omitted, not null")
	}
}
