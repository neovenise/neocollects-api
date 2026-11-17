package collection

import (
	"strings"
	"testing"
)

func TestListPagesWithACursorAndFiltersBySet(t *testing.T) {
	a := newAPI(t)
	for _, card := range []string{alpha, beta, gamma, delta} {
		a.call(a.ann, "POST", "/v1/me/collection/entries", body(card, "mint", "FR", 1))
	}

	page1 := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/entries?limit=3", ""))
	items1 := page1["items"].([]any)
	cursor, _ := page1["nextCursor"].(string)
	page2 := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/entries?limit=3&cursor="+cursor, ""))
	set := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/entries?setId=zzt2", ""))

	if len(items1) != 3 || cursor == "" {
		t.Fatalf("page 1 = %v", page1)
	}
	if len(page2["items"].([]any)) != 1 || page2["nextCursor"] != nil {
		t.Errorf("page 2 = %v, want one item and no cursor", page2)
	}
	if items := set["items"].([]any); len(items) != 1 || items[0].(map[string]any)["cardId"] != delta {
		t.Errorf("set filter = %v", set)
	}
	for _, bad := range []string{"limit=0", "limit=x", "cursor=!!!", "cursor=" + "bm9wZQ"} {
		if rec := a.call(a.ann, "GET", "/v1/me/collection/entries?"+bad, ""); rec.Code != 400 {
			t.Errorf("?%s = %d, want 400", bad, rec.Code)
		}
	}
}

func TestSummaryShape(t *testing.T) {
	a := newAPI(t)
	empty := a.call(a.ann, "GET", "/v1/me/collection/summary", "")
	a.call(a.ann, "POST", "/v1/me/collection/entries", body(delta, "mint", "FR", 4))
	full := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/summary", ""))

	if !strings.Contains(empty.Body.String(), `"sets":[]`) {
		t.Errorf("empty summary = %s, want sets as an empty array (not null)", empty.Body.String())
	}
	sets := full["sets"].([]any)
	if full["totalQuantity"] != float64(4) || full["distinctCards"] != float64(1) || full["valueCents"] != float64(1000) ||
		full["unpricedCards"] != float64(0) || len(sets) != 1 || sets[0].(map[string]any)["setId"] != "zzt2" || sets[0].(map[string]any)["ownedCount"] != float64(1) {
		t.Errorf("summary = %v", full)
	}
}
