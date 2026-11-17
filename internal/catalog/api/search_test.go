package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestSearchPassesTheScanFilters(t *testing.T) {
	r := &fakeReader{cards: []catalog.CardRow{{ID: "30th-024", SetID: "30th", LocalID: "024", Name: "Pikachu"}}}

	rec := get(t, newHandlerEngine(r, ""), "/v1/cards?number=24&setSize=128&name=Pika&setId=30th&rarity=Rare&rarity=Commune&ids=a,%20b,c")

	f := r.cardFilter
	if f.Number == nil || *f.Number != 24 || f.SetSize == nil || *f.SetSize != 128 {
		t.Errorf("number/size = %v/%v", f.Number, f.SetSize)
	}
	if f.Name != "Pika" || len(f.SetIDs) != 1 || f.SetIDs[0] != "30th" {
		t.Errorf("name/set = %q/%v", f.Name, f.SetIDs)
	}
	if len(f.Rarities) != 2 || f.Rarities[0] != "Rare" || f.Rarities[1] != "Commune" {
		t.Errorf("rarities = %v", f.Rarities)
	}
	if len(f.IDs) != 3 || f.IDs[1] != "b" {
		t.Errorf("ids = %v", f.IDs)
	}
	if got := decode(t, rec)["nextCursor"]; got != nil {
		t.Errorf("nextCursor = %v, want null", got)
	}
}

func TestSearchLimitAndPagination(t *testing.T) {
	r := &fakeReader{cards: []catalog.CardRow{{ID: "a"}}, hasMore: true}
	h := newHandlerEngine(r, "")

	rec := get(t, h, "/v1/cards")
	if r.cardFilter.Limit != 50 || r.cardFilter.Offset != 0 {
		t.Errorf("defaults = limit %d, offset %d", r.cardFilter.Limit, r.cardFilter.Offset)
	}
	next, _ := decode(t, rec)["nextCursor"].(string)
	if next == "" {
		t.Fatal("more results: a nextCursor is expected")
	}

	get(t, h, "/v1/cards?limit=1000")
	if r.cardFilter.Limit != 200 {
		t.Errorf("limit must be capped at 200, got %d", r.cardFilter.Limit)
	}

	get(t, h, "/v1/cards?limit=20&cursor="+next)
	if r.cardFilter.Offset != 50 || r.cardFilter.Limit != 20 {
		t.Errorf("the cursor must carry the offset: limit %d, offset %d", r.cardFilter.Limit, r.cardFilter.Offset)
	}
	nextOfNext, _ := decode(t, get(t, h, "/v1/cards?limit=20&cursor="+next))["nextCursor"].(string)
	if nextOfNext == "" || nextOfNext == next {
		t.Errorf("the next cursor must move on: %q after %q", nextOfNext, next)
	}
}

func TestExactNameIsPassedToTheSearchAndLimitedInLength(t *testing.T) {
	r := &fakeReader{}
	h := newHandlerEngine(r, "")

	if rec := get(t, h, "/v1/cards?exactName=Dracaufeu-ex&limit=100"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if r.cardFilter.ExactName != "Dracaufeu-ex" || r.cardFilter.Name != "" {
		t.Errorf("filter = %+v", r.cardFilter)
	}
	if rec := get(t, h, "/v1/cards?exactName="+strings.Repeat("a", 101)); rec.Code != http.StatusBadRequest {
		t.Errorf("a name that long must be refused, got %d", rec.Code)
	}
}

func TestSearchPassesTheTextAndEveryAttributeFilter(t *testing.T) {
	r := &fakeReader{}

	get(t, newHandlerEngine(r, ""), "/v1/cards?q=kach&type=Feu&type=Eau&category=Pok%C3%A9mon&stage=Base&trainerType=Objet&regulationMark=G&setId=sv03&setId=30th&rarity=Rare")

	f := r.cardFilter
	if f.Query != "kach" || len(f.Types) != 2 || f.Types[1] != "Eau" || len(f.Categories) != 1 || f.Categories[0] != "Pokémon" {
		t.Errorf("filter = %+v", f)
	}
	if len(f.Stages) != 1 || len(f.TrainerTypes) != 1 || len(f.RegulationMarks) != 1 || len(f.SetIDs) != 2 || len(f.Rarities) != 1 {
		t.Errorf("filter = %+v", f)
	}
}

func TestSearchRefusesAnEmptyOrTooManyFilterValues(t *testing.T) {
	h := newHandlerEngine(&fakeReader{}, "")
	many := strings.Repeat("&type=x", maxFilterValues+1)

	for name, target := range map[string]string{
		"empty type": "/v1/cards?type=", "empty rarity": "/v1/cards?rarity=Rare&rarity=", "too many": "/v1/cards?" + many[1:],
		"long value": "/v1/cards?stage=" + strings.Repeat("a", maxTextParam+1), "long q": "/v1/cards?q=" + strings.Repeat("a", maxTextParam+1),
	} {
		rec := get(t, h, target)
		if rec.Code != http.StatusBadRequest || decode(t, rec)["code"] != "invalid_parameter" {
			t.Errorf("%s: %d %s, want 400 invalid_parameter", name, rec.Code, rec.Body.String())
		}
	}
}

func TestFacetsAreServedAndCached(t *testing.T) {
	r := &fakeReader{facets: catalog.Facets{
		Rarities: []catalog.FacetValue{{Value: "Commune", Count: 3}}, Types: []catalog.FacetValue{{Value: "Feu", Count: 2}},
	}}

	rec := get(t, newHandlerEngine(r, ""), "/v1/cards/facets")

	if rec.Code != http.StatusOK || rec.Header().Get("ETag") == "" || !strings.Contains(rec.Header().Get("Cache-Control"), "max-age") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	body := decode(t, rec)
	rarities, _ := body["rarities"].([]any)
	if len(rarities) != 1 || rarities[0].(map[string]any)["value"] != "Commune" || rarities[0].(map[string]any)["count"] != float64(3) {
		t.Errorf("body = %s", rec.Body.String())
	}
	if rec := get(t, newHandlerEngine(&fakeReader{err: errors.New("boom")}, ""), "/v1/cards/facets"); rec.Code != http.StatusInternalServerError {
		t.Errorf("a failing database answers %d, want 500", rec.Code)
	}
}

func TestSearchPassesTheIllustratorAndLimitsItsLength(t *testing.T) {
	r := &fakeReader{}

	get(t, newHandlerEngine(r, ""), "/v1/cards?illustrator=%20Satoshi%20Ito%20")
	rec := get(t, newHandlerEngine(r, ""), "/v1/cards?illustrator="+strings.Repeat("a", 101))

	if r.cardFilter.Illustrator != "Satoshi Ito" {
		t.Errorf("illustrator = %q", r.cardFilter.Illustrator)
	}
	expectProblem(t, rec, http.StatusBadRequest, "invalid_parameter")
}
