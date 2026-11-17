package lists

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPuttingACardInAListIsIdempotentAndRemovingIt(t *testing.T) {
	a := newAPI(t)
	favorites := a.listOfKind(a.ann, "favorites")["id"].(string)
	url := "/v1/me/lists/" + favorites + "/cards/" + alpha

	first := a.call(a.ann, "PUT", url, "")
	again := a.call(a.ann, "PUT", url, "")
	count := a.listOfKind(a.ann, "favorites")["cardCount"]
	removed := a.call(a.ann, "DELETE", url, "")
	removedAgain := a.call(a.ann, "DELETE", url, "")

	for name, rec := range map[string]*httptest.ResponseRecorder{"put": first, "put again": again, "delete": removed, "delete again": removedAgain} {
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Errorf("%s: %d %q, want 204", name, rec.Code, rec.Body.String())
		}
	}
	if count != float64(1) {
		t.Errorf("count after two puts = %v, want 1", count)
	}
	if got := a.listOfKind(a.ann, "favorites")["cardCount"]; got != float64(0) {
		t.Errorf("count after removal = %v", got)
	}
}

func TestAnUnknownCardIsNotFound(t *testing.T) {
	a := newAPI(t)
	favorites := a.listOfKind(a.ann, "favorites")["id"].(string)

	for _, card := range []string{"nope-1", digital} {
		rec := a.call(a.ann, "PUT", "/v1/me/lists/"+favorites+"/cards/"+card, "")
		if rec.Code != 404 || object(t, rec)["code"] != "card_not_found" {
			t.Errorf("%s: %d %s, want 404 card_not_found", card, rec.Code, rec.Body.String())
		}
	}
}

func TestAnotherAccountsListAnAbsentOneAndAMalformedIdAreAllNotFound(t *testing.T) {
	a := newAPI(t)
	custom := object(t, a.call(a.ann, "POST", "/v1/me/lists", `{"name": "Secrète"}`))["id"].(string)
	a.call(a.ann, "PUT", "/v1/me/lists/"+custom+"/cards/"+alpha, "")

	for name, id := range map[string]string{
		"ann's, as bob": custom, "absent": "3f2b8c1e-0000-4000-8000-000000000000", "not an id": "1;DROP",
	} {
		calls := []struct{ method, path, body string }{
			{"PATCH", "/v1/me/lists/" + id, `{"name": "x"}`}, {"DELETE", "/v1/me/lists/" + id, ""},
			{"PUT", "/v1/me/lists/" + id + "/cards/" + beta, ""}, {"DELETE", "/v1/me/lists/" + id + "/cards/" + alpha, ""},
		}
		for _, c := range calls {
			rec := a.call(a.bob, c.method, c.path, c.body)
			if rec.Code != 404 || object(t, rec)["code"] != "list_not_found" {
				t.Errorf("%s %s (%s): %d %s, want 404 list_not_found", c.method, c.path, name, rec.Code, rec.Body.String())
			}
		}
	}
	own := a.listOfKind(a.ann, "custom")
	if own["name"] != "Secrète" || own["cardCount"] != float64(1) {
		t.Errorf("ann's list was changed by bob: %v", own)
	}
	if n := len(a.items(a.bob, "")); n != 3 {
		t.Errorf("bob sees %d lists, want his own 3", n)
	}
}

func TestAListCanAskForSeveralCopiesAndTheQuantityCanBeChanged(t *testing.T) {
	a := newAPI(t)
	wishlist := a.listOfKind(a.ann, "wishlist")["id"].(string)
	url := "/v1/me/lists/" + wishlist + "/cards/" + alpha

	three := a.call(a.ann, "PUT", url, `{"quantity": 3}`)
	got := a.listOfKind(a.ann, "wishlist")["cards"].([]any)[0].(map[string]any)
	a.call(a.ann, "PUT", url, `{"quantity": 1}`)
	back := a.listOfKind(a.ann, "wishlist")["cards"].([]any)[0].(map[string]any)

	if three.Code != http.StatusNoContent || got["quantity"] != float64(3) {
		t.Errorf("put with 3: %d, card %v", three.Code, got)
	}
	if back["quantity"] != float64(1) {
		t.Errorf("quantity after asking for 1 = %v: the quantity is set, not added", back["quantity"])
	}
	if n := a.listOfKind(a.ann, "wishlist")["cardCount"]; n != float64(1) {
		t.Errorf("cardCount = %v: it counts different cards, not copies", n)
	}
}

func TestPuttingACardAgainWithoutAQuantityKeepsTheQuantity(t *testing.T) {
	a := newAPI(t)
	trade := a.listOfKind(a.ann, "trade")["id"].(string)
	url := "/v1/me/lists/" + trade + "/cards/" + alpha
	a.call(a.ann, "PUT", url, `{"quantity": 4}`)

	for _, body := range []string{"", `{}`, "  "} {
		if rec := a.call(a.ann, "PUT", url, body); rec.Code != http.StatusNoContent {
			t.Fatalf("put %q: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	card := a.listOfKind(a.ann, "trade")["cards"].([]any)[0].(map[string]any)
	if card["quantity"] != float64(4) {
		t.Errorf("quantity = %v, want 4: "+"making sure a card is there must not reset what it asks for", card["quantity"])
	}
}

func TestAQuantityOutOfRangeOrABadBodyIsRefusedAndChangesNothing(t *testing.T) {
	a := newAPI(t)
	favorites := a.listOfKind(a.ann, "favorites")["id"].(string)
	url := "/v1/me/lists/" + favorites + "/cards/" + alpha
	a.call(a.ann, "PUT", url, `{"quantity": 2}`)

	for name, body := range map[string]string{
		"zero": `{"quantity": 0}`, "negative": `{"quantity": -1}`, "huge": `{"quantity": 100}`,
	} {
		rec := a.call(a.ann, "PUT", url, body)
		if rec.Code != 422 || object(t, rec)["code"] != "invalid_quantity" {
			t.Errorf("%s: %d %s, want 422 invalid_quantity", name, rec.Code, rec.Body.String())
		}
	}
	if rec := a.call(a.ann, "PUT", url, `nope`); rec.Code != 400 || object(t, rec)["code"] != "invalid_body" {
		t.Errorf("not json: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.call(a.ann, "PUT", url, `{"quantity": 99}`); rec.Code != http.StatusNoContent {
		t.Errorf("99 is the maximum and is allowed: %d", rec.Code)
	}
	a.call(a.ann, "PUT", url, `{"quantity": 2}`)
	if q := a.listOfKind(a.ann, "favorites")["cards"].([]any)[0].(map[string]any)["quantity"]; q != float64(2) {
		t.Errorf("quantity = %v, want 2", q)
	}
}
