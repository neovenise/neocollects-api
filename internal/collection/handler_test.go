package collection

import (
	"net/http"
	"testing"
)

func TestEveryRouteNeedsAnAccount(t *testing.T) {
	a := newAPI(t)
	routes := [][2]string{
		{"GET", "/v1/me/collection/entries"}, {"POST", "/v1/me/collection/entries"},
		{"PATCH", "/v1/me/collection/entries/3f2b8c1e-0000-4000-8000-000000000000"},
		{"DELETE", "/v1/me/collection/entries/3f2b8c1e-0000-4000-8000-000000000000"}, {"GET", "/v1/me/collection/summary"},
	}
	for _, r := range routes {
		rec := a.call("", r[0], r[1], `{}`)
		if rec.Code != http.StatusUnauthorized || decodeObject(t, rec)["code"] != "invalid_token" {
			t.Errorf("%s %s without a token = %d %s, want 401 invalid_token", r[0], r[1], rec.Code, rec.Body.String())
		}
	}
	if rec := a.call("not-a-real-token", "GET", "/v1/me/collection/summary", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("a made-up token = %d, want 401", rec.Code)
	}
}

func TestEachAccountSeesOnlyItsOwnCollection(t *testing.T) {
	a := newAPI(t)
	a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 5))
	a.call(a.bob, "POST", "/v1/me/collection/entries", body(gamma, "mint", "EN", 1))

	annList := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/entries", ""))["items"].([]any)
	bobList := decodeObject(t, a.call(a.bob, "GET", "/v1/me/collection/entries", ""))["items"].([]any)

	if len(annList) != 1 || annList[0].(map[string]any)["cardId"] != alpha || len(bobList) != 1 || bobList[0].(map[string]any)["cardId"] != gamma {
		t.Errorf("ann sees %v, bob sees %v", annList, bobList)
	}
}
