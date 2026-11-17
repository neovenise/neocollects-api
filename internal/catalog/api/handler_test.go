package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestInvalidParametersAreProblems(t *testing.T) {
	h := newHandlerEngine(&fakeReader{}, "")
	tests := []struct {
		path string
		code string
	}{
		{"/v1/sets?officialCount=abc", "invalid_parameter"},
		{"/v1/sets?officialCount=-1", "invalid_parameter"},
		{"/v1/cards?number=1.5", "invalid_parameter"},
		{"/v1/cards?setSize=x", "invalid_parameter"},
		{"/v1/cards?limit=0", "invalid_parameter"},
		{"/v1/cards?limit=abc", "invalid_parameter"},
		{"/v1/cards?ids=,", "invalid_parameter"},
		{"/v1/cards?cursor=garbage", "invalid_cursor"},
		{"/v1/cards?cursor=" + encodeBase64("offset:-3"), "invalid_cursor"},
		{"/v1/cards?name=" + strings.Repeat("a", 101), "invalid_parameter"},
		{"/v1/cards?rarity=" + strings.Repeat("a", 101), "invalid_parameter"},
		{"/v1/cards/x/price-history?from=yesterday", "invalid_parameter"},
		{"/v1/cards/x/price-history?to=2026-13-40", "invalid_parameter"},
		{"/v1/blocks?lang=en", "unsupported_language"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			expectProblem(t, get(t, h, tt.path), http.StatusBadRequest, tt.code)
		})
	}
	if rec := get(t, h, "/v1/blocks?lang=fr"); rec.Code != 200 {
		t.Errorf("lang=fr must be accepted: %d", rec.Code)
	}
}

func encodeBase64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func TestTooManyIDs(t *testing.T) {
	ids := make([]string, maxIDs+1)
	for i := range ids {
		ids[i] = "a"
	}

	expectProblem(t, get(t, newHandlerEngine(&fakeReader{}, ""), "/v1/cards?ids="+strings.Join(ids, ",")), 400, "too_many_ids")
}

func TestNotFoundErrors(t *testing.T) {
	h := newHandlerEngine(&fakeReader{notFound: true}, "https://img.test")
	tests := map[string]string{
		"/v1/sets/nope":                "set_not_found",
		"/v1/sets/nope/cards":          "set_not_found",
		"/v1/cards/nope":               "card_not_found",
		"/v1/cards/nope/foil":          "foil_not_found",
		"/v1/cards/nope/price-history": "card_not_found",
	}
	for path, code := range tests {
		t.Run(path, func(t *testing.T) { expectProblem(t, get(t, h, path), http.StatusNotFound, code) })
	}
}

func TestInternalErrorsDoNotLeak(t *testing.T) {
	h := newHandlerEngine(&fakeReader{err: errors.New(`pq: relation "secret_table" does not exist`)}, "")

	rec := get(t, h, "/v1/blocks")

	expectProblem(t, rec, http.StatusInternalServerError, "internal_error")
	if strings.Contains(rec.Body.String(), "secret_table") {
		t.Errorf("the cause leaked: %s", rec.Body.String())
	}
}
