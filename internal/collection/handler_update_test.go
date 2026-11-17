package collection

import (
	"net/http"
	"testing"
)

func TestPatchingChangesTheEntryAndMergesWhenItBecomesAnExistingOne(t *testing.T) {
	a := newAPI(t)
	mint := decodeObject(t, a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 2)))
	played := decodeObject(t, a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "played", "FR", 1)))

	qty := a.call(a.ann, "PATCH", "/v1/me/collection/entries/"+played["id"].(string), `{"quantity": 4}`)
	merge := a.call(a.ann, "PATCH", "/v1/me/collection/entries/"+played["id"].(string), `{"condition": "mint"}`)

	if qty.Code != 200 || decodeObject(t, qty)["quantity"] != float64(4) {
		t.Errorf("quantity patch: %d %s", qty.Code, qty.Body.String())
	}
	m := decodeObject(t, merge)
	if merge.Code != 200 || m["id"] != mint["id"] || m["quantity"] != float64(6) {
		t.Errorf("merge patch: %d %s, want the existing mint entry with 6", merge.Code, merge.Body.String())
	}
	if gone := a.call(a.ann, "DELETE", "/v1/me/collection/entries/"+played["id"].(string), ""); gone.Code != 404 {
		t.Errorf("the merged entry should be gone, delete = %d", gone.Code)
	}
}

func TestPatchingValidatesAndNeedsSomethingToChange(t *testing.T) {
	a := newAPI(t)
	e := decodeObject(t, a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1)))
	url := "/v1/me/collection/entries/" + e["id"].(string)
	cases := []struct {
		body   string
		status int
		code   string
	}{
		{`{}`, 400, "invalid_body"},
		{`nope`, 400, "invalid_body"},
		{`{"quantity": 0}`, 422, "invalid_quantity"},
		{`{"condition": "shiny"}`, 422, "invalid_condition"},
		{`{"language": "french"}`, 422, "invalid_language"},
	}
	for _, c := range cases {
		rec := a.call(a.ann, "PATCH", url, c.body)
		if rec.Code != c.status || decodeObject(t, rec)["code"] != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.body, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
}

func TestAnotherAccountsEntryAnAbsentOneAndAMalformedIdAreAllNotFound(t *testing.T) {
	a := newAPI(t)
	e := decodeObject(t, a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1)))
	annsURL := "/v1/me/collection/entries/" + e["id"].(string)

	for name, url := range map[string]string{
		"ann's entry, as bob": annsURL, "absent": "/v1/me/collection/entries/3f2b8c1e-0000-4000-8000-000000000000", "not an id": "/v1/me/collection/entries/1;DROP",
	} {
		for _, call := range []struct{ method, body string }{{"PATCH", `{"quantity": 9}`}, {"DELETE", ""}} {
			rec := a.call(a.bob, call.method, url, call.body)
			if rec.Code != 404 || decodeObject(t, rec)["code"] != "entry_not_found" {
				t.Errorf("%s %s: %d %s, want 404 entry_not_found", call.method, name, rec.Code, rec.Body.String())
			}
		}
	}
	if own := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/summary", "")); own["totalQuantity"] != float64(1) {
		t.Errorf("ann's collection was changed by bob: %v", own)
	}
}

func TestDeleteAnswers204(t *testing.T) {
	a := newAPI(t)
	e := decodeObject(t, a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1)))

	rec := a.call(a.ann, "DELETE", "/v1/me/collection/entries/"+e["id"].(string), "")

	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("%d %q", rec.Code, rec.Body.String())
	}
}
