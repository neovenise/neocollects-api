package lists

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEveryRouteNeedsAnAccount(t *testing.T) {
	a := newAPI(t)
	id := "3f2b8c1e-0000-4000-8000-000000000000"
	routes := [][2]string{
		{"GET", "/v1/me/lists"}, {"POST", "/v1/me/lists"}, {"PATCH", "/v1/me/lists/" + id}, {"DELETE", "/v1/me/lists/" + id},
		{"PUT", "/v1/me/lists/" + id + "/cards/" + alpha}, {"DELETE", "/v1/me/lists/" + id + "/cards/" + alpha},
	}
	for _, r := range routes {
		rec := a.call("", r[0], r[1], `{}`)
		if rec.Code != http.StatusUnauthorized || object(t, rec)["code"] != "invalid_token" {
			t.Errorf("%s %s without a token = %d, want 401 invalid_token", r[0], r[1], rec.Code)
		}
	}
}

func TestListingAnswersTheDefaultListsWithoutCardsUnlessAsked(t *testing.T) {
	a := newAPI(t)
	a.call(a.ann, "PUT", "/v1/me/lists/"+a.listOfKind(a.ann, "favorites")["id"].(string)+"/cards/"+alpha, "")

	plain := a.call(a.ann, "GET", "/v1/me/lists", "")
	items := a.items(a.ann, "")
	withIDs := a.listOfKind(a.ann, "favorites")

	if cc := plain.Header().Get("Cache-Control"); !strings.Contains(cc, "private") || !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q: lists are personal", cc)
	}
	if len(items) != 3 || items[0]["kind"] != "favorites" || items[0]["cardCount"] != float64(1) {
		t.Fatalf("items = %v", items)
	}
	if _, has := items[0]["cards"]; has {
		t.Error("cards must be absent unless include=cards")
	}
	cards, _ := withIDs["cards"].([]any)
	if len(cards) != 1 || cards[0].(map[string]any)["cardId"] != alpha || cards[0].(map[string]any)["quantity"] != float64(1) {
		t.Errorf("cards = %v, want alpha with a quantity of 1", withIDs["cards"])
	}
	if prev, _ := items[1]["previewCardIds"].([]any); prev == nil {
		t.Errorf("previewCardIds of an empty list = %v, want [] and not null", items[1]["previewCardIds"])
	}
}

func TestCreatingAListAnswers201AndValidatesTheName(t *testing.T) {
	a := newAPI(t)

	created := a.call(a.ann, "POST", "/v1/me/lists", `{"name": "  Mes holos  "}`)

	if created.Code != http.StatusCreated {
		t.Fatalf("%d %s", created.Code, created.Body.String())
	}
	if l := object(t, created); l["name"] != "Mes holos" || l["kind"] != "custom" || l["cardCount"] != float64(0) || l["id"] == "" {
		t.Errorf("list = %v (the name is trimmed)", l)
	}
	for name, body := range map[string]string{
		"not json": `nope`, "empty": `{"name": ""}`, "blank": `{"name": "   "}`, "missing": `{}`,
		"too long": `{"name": "` + strings.Repeat("é", 61) + `"}`, "control": `{"name": "a\nb"}`,
	} {
		rec := a.call(a.ann, "POST", "/v1/me/lists", body)
		want := "invalid_name"
		if name == "not json" {
			want = "invalid_body"
		}
		if rec.Code < 400 || object(t, rec)["code"] != want {
			t.Errorf("%s: %d %s, want %s", name, rec.Code, rec.Body.String(), want)
		}
	}
	if n := len(a.items(a.ann, "")); n != 4 {
		t.Errorf("%d lists, want 4: refused names must create nothing", n)
	}
	if rec := a.call(a.ann, "POST", "/v1/me/lists", `{"name": "`+strings.Repeat("é", 60)+`"}`); rec.Code != http.StatusCreated {
		t.Errorf("60 characters (not bytes) are allowed: %d", rec.Code)
	}
}

func TestRenamingAndDeletingACustomListButNotADefaultOne(t *testing.T) {
	a := newAPI(t)
	custom := object(t, a.call(a.ann, "POST", "/v1/me/lists", `{"name": "À moi"}`))["id"].(string)
	favorites := a.listOfKind(a.ann, "favorites")["id"].(string)

	renamed := a.call(a.ann, "PATCH", "/v1/me/lists/"+custom, `{"name": "Aux autres"}`)
	locked := a.call(a.ann, "PATCH", "/v1/me/lists/"+favorites, `{"name": "Autre"}`)
	lockedDelete := a.call(a.ann, "DELETE", "/v1/me/lists/"+favorites, "")
	deleted := a.call(a.ann, "DELETE", "/v1/me/lists/"+custom, "")
	again := a.call(a.ann, "DELETE", "/v1/me/lists/"+custom, "")

	if renamed.Code != 200 || object(t, renamed)["name"] != "Aux autres" {
		t.Errorf("rename: %d %s", renamed.Code, renamed.Body.String())
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{"rename": locked, "delete": lockedDelete} {
		if rec.Code != http.StatusForbidden || object(t, rec)["code"] != "default_list" {
			t.Errorf("default list %s: %d %s, want 403 default_list", name, rec.Code, rec.Body.String())
		}
	}
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Errorf("delete: %d %q", deleted.Code, deleted.Body.String())
	}
	if again.Code != http.StatusNotFound || object(t, again)["code"] != "list_not_found" {
		t.Errorf("second delete: %d %s", again.Code, again.Body.String())
	}
}
