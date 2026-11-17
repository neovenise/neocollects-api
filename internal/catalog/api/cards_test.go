package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestCardDetailShape(t *testing.T) {

	updated := time.Date(2026, 11, 14, 23, 24, 7, 0, time.FixedZone("CET", 3600))
	r := &fakeReader{detail: catalog.CardDetailRow{
		CardRow: catalog.CardRow{ID: "30th-024", SetID: "30th", LocalID: "024", Name: "Pikachu", Rarity: "Pikachu Rare",
			ImageKey: str("cards/30th-024.webp"), ThumbKey: str("cards/30th-024.thumb.webp")},
		Set:        catalog.SetRow{ID: "30th", Code: "30C", Name: "30ᵉ Anniversaire", BlockID: "me", BlockName: "Méga-Évolution", OfficialCount: 128, TotalCount: 161},
		PriceCents: new(int), PriceCurrency: str("EUR"), PriceUpdatedAt: &updated,
	}}
	*r.detail.PriceCents = 56

	card := decode(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/30th-024"))

	if card["id"] != "30th-024" || card["localId"] != "024" || card["setId"] != "30th" || card["rarity"] != "Pikachu Rare" {
		t.Errorf("card = %v", card)
	}
	if card["imageUrl"] != "https://img.test/cards/30th-024.webp" || card["thumbUrl"] != "https://img.test/cards/30th-024.thumb.webp" {
		t.Errorf("images = %v / %v", card["imageUrl"], card["thumbUrl"])
	}
	if card["priceCents"] != float64(56) || card["priceCurrency"] != "EUR" || card["priceUpdatedAt"] != "2026-11-14T22:24:07Z" {
		t.Errorf("price = %v %v %v", card["priceCents"], card["priceCurrency"], card["priceUpdatedAt"])
	}
	if set := card["set"].(map[string]any); set["officialCount"] != float64(128) || set["id"] != "30th" {
		t.Errorf("set = %v", set)
	}
}

func TestCardDetailCarriesThePriceAveragesAndOmitsTheMissingOnes(t *testing.T) {
	updated := time.Date(2026, 11, 14, 22, 24, 7, 0, time.UTC)
	cents, day, week := 402, 365, 376
	r := &fakeReader{detail: catalog.CardDetailRow{
		CardRow:    catalog.CardRow{ID: "sv03-125", Name: "Dracaufeu-ex"},
		PriceCents: &cents, PriceCurrency: str("EUR"), PriceUpdatedAt: &updated,
		PriceAvg1Cents: &day, PriceAvg7Cents: &week,
	}}

	card := decode(t, get(t, newHandlerEngine(r, ""), "/v1/cards/sv03-125"))

	avg, ok := card["priceAverages"].(map[string]any)
	if !ok || avg["day"] != float64(365) || avg["week"] != float64(376) {
		t.Fatalf("priceAverages = %v", card["priceAverages"])
	}
	if _, has := avg["month"]; has {
		t.Errorf("a missing average must be omitted, not null: %v", avg)
	}
}

func TestCardWithoutAveragesHasNoAveragesObject(t *testing.T) {
	cents := 402
	r := &fakeReader{detail: catalog.CardDetailRow{CardRow: catalog.CardRow{ID: "x-1", Name: "X"}, PriceCents: &cents, PriceCurrency: str("EUR")}}

	card := decode(t, get(t, newHandlerEngine(r, ""), "/v1/cards/x-1"))

	if _, has := card["priceAverages"]; has {
		t.Errorf("priceAverages must be omitted when none is known: %v", card["priceAverages"])
	}
}

func TestCardWithoutPriceNorPictureOmitsThem(t *testing.T) {
	r := &fakeReader{detail: catalog.CardDetailRow{CardRow: catalog.CardRow{ID: "x-1", Name: "Sans rien"}}}

	card := decode(t, get(t, newHandlerEngine(r, ""), "/v1/cards/x-1"))

	for _, key := range []string{"priceCents", "priceCurrency", "priceUpdatedAt", "imageUrl", "thumbUrl"} {
		if _, has := card[key]; has {
			t.Errorf("%s must be omitted, not null: %v", key, card[key])
		}
	}
}

func TestFoil(t *testing.T) {
	r := &fakeReader{foil: catalog.FoilRow{Look: "ex", FrontKey: str("f/front.webp"), MaskKey: str("f/mask.png"), EtchKey: str("f/etch.webp")}}

	foil := decode(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/sv03-125/foil"))

	if foil["look"] != "ex" || foil["frontUrl"] != "https://img.test/f/front.webp" ||
		foil["maskUrl"] != "https://img.test/f/mask.png" || foil["etchUrl"] != "https://img.test/f/etch.webp" {
		t.Errorf("foil = %v", foil)
	}

	expectProblem(t, get(t, newHandlerEngine(r, ""), "/v1/cards/sv03-125/foil"), http.StatusNotFound, "foil_not_found")

	r.foil.EtchKey = nil
	if _, has := decode(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/sv03-125/foil"))["etchUrl"]; has {
		t.Error("no etch: no etchUrl")
	}
}

func TestFoilUrlsComeFromOurStorage(t *testing.T) {
	r := &fakeReader{foil: catalog.FoilRow{
		Look: "ex", FrontKey: str("f/front.webp"), MaskKey: str("f/mask.png"), EtchKey: str("f/etch.png"),
	}}

	foil := decode(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/x/foil"))

	if foil["frontUrl"] != "https://img.test/f/front.webp" || foil["maskUrl"] != "https://img.test/f/mask.png" || foil["etchUrl"] != "https://img.test/f/etch.png" {
		t.Errorf("foil = %v", foil)
	}
}

func TestAFoilNotCopiedYetIsNoFoil(t *testing.T) {
	r := &fakeReader{foil: catalog.FoilRow{Look: "holo"}}

	expectProblem(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/x/foil"), http.StatusNotFound, "foil_not_found")
}

func TestAFoilWithoutAMaskIsNoFoil(t *testing.T) {
	r := &fakeReader{foil: catalog.FoilRow{Look: "holo", FrontKey: str("f/front.webp")}}

	expectProblem(t, get(t, newHandlerEngine(r, "https://img.test"), "/v1/cards/x/foil"), http.StatusNotFound, "foil_not_found")
}

func TestPriceHistory(t *testing.T) {
	r := &fakeReader{points: []catalog.PricePointRow{{Day: "2026-11-01", Cents: 100}, {Day: "2026-11-03", Cents: 150}}}

	rec := get(t, newHandlerEngine(r, ""), "/v1/cards/sv03-125/price-history?from=2026-11-01&to=2026-11-30")

	if r.from == nil || *r.from != "2026-11-01" || r.to == nil || *r.to != "2026-11-30" {
		t.Errorf("range = %v..%v", r.from, r.to)
	}
	if rec.Body.String() != `{"items":[{"date":"2026-11-01","cents":100},{"date":"2026-11-03","cents":150}]}` {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestTheIllustratorIsAnsweredWhenKnownAndLeftOutOtherwise(t *testing.T) {
	r := &fakeReader{detail: catalog.CardDetailRow{
		CardRow:     catalog.CardRow{ID: "30th-024", SetID: "30th", LocalID: "024", Name: "Pikachu"},
		Set:         catalog.SetRow{ID: "30th", Name: "30ᵉ Anniversaire", BlockID: "me", BlockName: "Méga-Évolution"},
		Illustrator: "Satoshi Ito",
	}}
	known := decode(t, get(t, newHandlerEngine(r, ""), "/v1/cards/30th-024"))

	r.detail.Illustrator = ""
	unknown := decode(t, get(t, newHandlerEngine(r, ""), "/v1/cards/30th-024"))

	if known["illustrator"] != "Satoshi Ito" {
		t.Errorf("illustrator = %v", known["illustrator"])
	}
	if _, present := unknown["illustrator"]; present {
		t.Error("an unknown illustrator must be left out, never null or empty")
	}
}
