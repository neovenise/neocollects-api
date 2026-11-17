package collection

import (
	"net/http"
	"strings"
	"testing"
)

func TestAddingAnswers201ThenMergesWith200AndNeverCaches(t *testing.T) {
	a := newAPI(t)

	created := a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "nearMint", "fr", 1))
	merged := a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "nearMint", "FR", 2))

	if created.Code != http.StatusCreated || merged.Code != http.StatusOK {
		t.Fatalf("status %d then %d, want 201 then 200 (%s)", created.Code, merged.Code, merged.Body.String())
	}
	e := decodeObject(t, merged)
	if e["quantity"] != float64(3) || e["language"] != "FR" || e["cardId"] != alpha || e["condition"] != "nearMint" || e["id"] == "" || e["addedAt"] == "" {
		t.Errorf("entry = %v (language is stored in capitals, the second add merged into the first)", e)
	}
	if cc := created.Header().Get("Cache-Control"); !strings.Contains(cc, "private") || !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q: a collection is personal", cc)
	}
}

func TestAddingDefaultsTheQuantityToOne(t *testing.T) {
	a := newAPI(t)

	rec := a.call(a.ann, "POST", "/v1/me/collection/entries", `{"cardId":"`+alpha+`","condition":"mint","language":"FR"}`)

	if rec.Code != http.StatusCreated || decodeObject(t, rec)["quantity"] != float64(1) {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestAddingValidatesWhatItIsGiven(t *testing.T) {
	a := newAPI(t)
	cases := []struct {
		name, body string
		status     int
		code       string
	}{
		{"not json", `nope`, 400, "invalid_body"},
		{"no card", `{"condition":"mint","language":"FR"}`, 422, "unknown_card"},
		{"unknown card", body("nope-1", "mint", "FR", 1), 422, "unknown_card"},
		{"digital card", body(digital, "mint", "FR", 1), 422, "unknown_card"},
		{"condition", body(alpha, "pristine", "FR", 1), 422, "invalid_condition"},
		{"no condition", `{"cardId":"` + alpha + `","language":"FR"}`, 422, "invalid_condition"},
		{"language", body(alpha, "mint", "FRA", 1), 422, "invalid_language"},
		{"language digits", body(alpha, "mint", "F1", 1), 422, "invalid_language"},
		{"quantity zero", body(alpha, "mint", "FR", 0), 422, "invalid_quantity"},
		{"quantity negative", body(alpha, "mint", "FR", -3), 422, "invalid_quantity"},
		{"quantity huge", body(alpha, "mint", "FR", MaxQuantity+1), 422, "invalid_quantity"},
	}
	for _, c := range cases {
		rec := a.call(a.ann, "POST", "/v1/me/collection/entries", c.body)
		if rec.Code != c.status || decodeObject(t, rec)["code"] != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
	if rec := a.call(a.ann, "GET", "/v1/me/collection/summary", ""); decodeObject(t, rec)["totalQuantity"] != float64(0) {
		t.Error("a refused request must add nothing")
	}
}

func TestMergingOverTheMaximumAnswers422(t *testing.T) {
	a := newAPI(t)
	a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", MaxQuantity))

	rec := a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1))

	if rec.Code != 422 || decodeObject(t, rec)["code"] != "quantity_too_large" {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestARetriedPostWithAnIdempotencyKeyIsAnsweredAgainWithoutAddingTwice(t *testing.T) {
	a := newAPI(t)
	key := []string{"Idempotency-Key", "5f0c-retry"}

	first := a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 2), key...)

	retry := a.call(a.ann, "POST", "/v1/me/collection/entries", `{ "quantity": 2, "language": "FR", "condition": "mint", "cardId": "`+alpha+`" }`, key...)

	if first.Code != 201 || retry.Code != 201 || retry.Header().Get("Idempotent-Replayed") != "true" || first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("first %d, retry %d replayed=%q", first.Code, retry.Code, retry.Header().Get("Idempotent-Replayed"))
	}
	if first.Body.String() != retry.Body.String() {
		t.Errorf("retry answered %s, first answered %s", retry.Body.String(), first.Body.String())
	}
	if s := decodeObject(t, a.call(a.ann, "GET", "/v1/me/collection/summary", "")); s["totalQuantity"] != float64(2) {
		t.Errorf("total = %v, want 2: the retry must not add again", s["totalQuantity"])
	}
}

func TestAKeyReusedForAnotherRequestAnswers422AndABadKeyAnswers400(t *testing.T) {
	a := newAPI(t)
	a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1), "Idempotency-Key", "k")

	reused := a.call(a.ann, "POST", "/v1/me/collection/entries", body(gamma, "mint", "FR", 1), "Idempotency-Key", "k")
	bad := a.call(a.ann, "POST", "/v1/me/collection/entries", body(alpha, "mint", "FR", 1), "Idempotency-Key", strings.Repeat("k", 129))

	if reused.Code != 422 || decodeObject(t, reused)["code"] != "idempotency_key_reused" {
		t.Errorf("reused: %d %s", reused.Code, reused.Body.String())
	}
	if bad.Code != 400 || decodeObject(t, bad)["code"] != "invalid_idempotency_key" {
		t.Errorf("bad key: %d %s", bad.Code, bad.Body.String())
	}
}
