package store

import (
	"context"
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func TestScanFindsThePikachuAndNeverThePocketOne(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()
	n24, size128, size30 := 24, 128, 30

	cards, _, err := q.SearchCards(ctx, catalog.CardFilter{Number: &n24, SetSize: &size128, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(cards); !equal(got, []string{"30th-024"}) {
		t.Errorf("024/128 = %v", got)
	}

	cards, _, _ = q.SearchCards(ctx, catalog.CardFilter{Number: &n24, SetSize: &size30, Limit: 10})
	if got := ids(cards); !equal(got, []string{"swsh-b-024", "swsh-a-024"}) {
		t.Errorf("024/30 = %v", got)
	}
}

func TestNameSearchIsAPrefixWithoutCaseNorAccents(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	search := func(name string) []string {
		cards, _, err := q.SearchCards(ctx, catalog.CardFilter{Name: name, Limit: 10})
		if err != nil {
			t.Fatalf("SearchCards(%q): %v", name, err)
		}
		return ids(cards)
	}

	if got := search("DRACAUFEU"); !equal(got, []string{"sv03-125"}) {
		t.Errorf("prefix, any case = %v", got)
	}
	if got := search("dracaufeu ex"); !equal(got, []string{"sv03-125"}) {
		t.Errorf("hyphen vs space = %v", got)
	}
	if got := search("ekai"); !equal(got, []string{"sv03-010"}) {
		t.Errorf("accents = %v", got)
	}
	if got := search("pikachu"); len(got) != 3 {
		t.Errorf("pikachu = %v", got)
	}
	if got := search("kachu"); len(got) != 0 {
		t.Errorf("a name search is a prefix, not a substring: %v", got)
	}
	for _, hostile := range []string{"%", "_", "---", `\`, "' OR '1'='1"} {
		if got := search(hostile); len(got) != 0 {
			t.Errorf("%q must find nothing, got %v", hostile, got)
		}
	}
}

func TestExactNameFindsEveryCardWithThatNameAndNothingLonger(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	exact := func(name string) []string {
		cards, _, err := q.SearchCards(ctx, catalog.CardFilter{ExactName: name, Limit: 10})
		if err != nil {
			t.Fatalf("SearchCards(%q): %v", name, err)
		}
		return ids(cards)
	}

	if got := exact("PIKACHU"); !equal(got, []string{"30th-024"}) {
		t.Errorf("pikachu = %v", got)
	}
	if got := exact("dracaufeu ex"); !equal(got, []string{"sv03-125"}) {
		t.Errorf("hyphen vs space = %v", got)
	}
	if got := exact("Pikach"); len(got) != 0 {
		t.Errorf("an exact name is not a prefix: %v", got)
	}
	for _, hostile := range []string{"%", "---", "' OR '1'='1"} {
		if got := exact(hostile); len(got) != 0 {
			t.Errorf("%q must find nothing, got %v", hostile, got)
		}
	}
}

func TestSearchByRaritiesIDsAndSet(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	cards, _, _ := q.SearchCards(ctx, catalog.CardFilter{Rarities: []string{"Double rare", "Commune"}, Limit: 10})
	if got := ids(cards); !equal(got, []string{"30th-001", "sv03-125"}) {
		t.Errorf("rarities = %v", got)
	}
	cards, _, _ = q.SearchCards(ctx, catalog.CardFilter{IDs: []string{"sv03-125", "tcgp-a1-024", "ghost"}, Limit: 10})
	if got := ids(cards); !equal(got, []string{"sv03-125"}) {
		t.Errorf("ids = %v (unknown and digital ones are skipped)", got)
	}
	cards, _, _ = q.SearchCards(ctx, catalog.CardFilter{SetIDs: []string{"sv03"}, Rarities: []string{"Rare"}, Limit: 10})
	if got := ids(cards); !equal(got, []string{"sv03-010"}) {
		t.Errorf("set + rarity = %v", got)
	}
}

func TestSearchPagination(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()

	page1, more1, err := q.SearchCards(ctx, catalog.CardFilter{Limit: 3, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	page2, more2, _ := q.SearchCards(ctx, catalog.CardFilter{Limit: 3, Offset: 3})
	page3, more3, _ := q.SearchCards(ctx, catalog.CardFilter{Limit: 3, Offset: 6})

	if len(page1) != 3 || !more1 || len(page2) != 3 || !more2 {
		t.Errorf("pages = %d/%v, %d/%v", len(page1), more1, len(page2), more2)
	}
	if len(page3) != 1 || more3 {
		t.Errorf("last page = %d cards, more = %v", len(page3), more3)
	}
	seen := map[string]bool{}
	for _, c := range append(append(page1, page2...), page3...) {
		if seen[c.ID] {
			t.Errorf("%s appears twice across pages", c.ID)
		}
		seen[c.ID] = true
	}
	if page1[0].SetID != "30th" {
		t.Errorf("the newest set comes first, got %s", page1[0].SetID)
	}
}

func TestSearchByPartOfTheNameFindsItAnywhereWithoutCaseOrAccents(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	for _, typed := range []string{"kach", "KACH", "ikachu"} {
		if got := search(t, q, catalog.CardFilter{Query: typed}); !sameSet(got, "30th-024", "swsh-a-024", "swsh-b-024") {
			t.Errorf("q=%q found %v, want the three physical Pikachu (the digital one never)", typed, got)
		}
	}
	for _, typed := range []string{"ekaiser", "ÉKAÏS", "kaï"} {
		if got := search(t, q, catalog.CardFilter{Query: typed}); !sameSet(got, "sv03-010") {
			t.Errorf("q=%q found %v, want Ékaïser", typed, got)
		}
	}
}

func TestSearchByPartOfTheNameShowsTheNamesThatBeginWithItFirst(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	got := search(t, q, catalog.CardFilter{Query: "e"})

	if len(got) < 3 || got[0] != "sv03-010" {
		t.Errorf("q=e found %v: Ékaïser begins with an e and must come first", got)
	}
}

func TestSearchByPartOfTheNameTreatsPercentAndSymbolsAsText(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	for _, typed := range []string{"%", "_", "---"} {
		if got := search(t, q, catalog.CardFilter{Query: typed}); len(got) != 0 {
			t.Errorf("q=%q found %v, want nothing (and not every card)", typed, got)
		}
	}
}

func TestSearchByTypeCategoryStageTrainerTypeAndMark(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	cases := []struct {
		name string
		f    catalog.CardFilter
		want []string
	}{
		{"one type", catalog.CardFilter{Types: []string{"Obscurité"}}, []string{"sv03-125"}},
		{"a second type of a dual-type Pokémon", catalog.CardFilter{Types: []string{"Psy"}}, []string{"30th-001"}},
		{"any of several types", catalog.CardFilter{Types: []string{"Plante", "Électrique"}}, []string{"30th-001", "30th-024"}},
		{"category", catalog.CardFilter{Categories: []string{"Dresseur"}}, []string{"30th-TG01"}},
		{"stage", catalog.CardFilter{Stages: []string{"Niveau 1", "Niveau 2"}}, []string{"sv03-010", "sv03-125"}},
		{"kind of trainer", catalog.CardFilter{TrainerTypes: []string{"Objet"}}, []string{"30th-TG01"}},
		{"regulation mark", catalog.CardFilter{RegulationMarks: []string{"G"}}, []string{"sv03-010", "sv03-125"}},
		{"filters together are an AND", catalog.CardFilter{Categories: []string{"Pokémon"}, Types: []string{"Plante"}, RegulationMarks: []string{"I"}}, []string{"30th-001"}},
		{"an AND that nothing satisfies", catalog.CardFilter{Types: []string{"Plante"}, RegulationMarks: []string{"G"}}, []string{}},
		{"text and filters together", catalog.CardFilter{Query: "e", Rarities: []string{"Rare"}, SetIDs: []string{"sv03"}}, []string{"sv03-010"}},
		{"several sets", catalog.CardFilter{SetIDs: []string{"sv03", "30th"}, Types: []string{"Métal", "Électrique"}}, []string{"sv03-010", "30th-024"}},
	}
	for _, c := range cases {
		if got := search(t, q, c.f); !sameSet(got, c.want...) {
			t.Errorf("%s: found %v, want %v", c.name, got, c.want)
		}
	}
}

func TestASearchPutsTheCardsWithoutAPictureLastInTheCatalogOrder(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	got := search(t, q, catalog.CardFilter{SetIDs: []string{"30th"}})

	if !equal(got, []string{"30th-024", "30th-001", "30th-TG01"}) {
		t.Errorf("found %v, want the card with a picture, then the others by number", got)
	}
}

func TestPicturelessCardsStayLastWhateverTheRelevanceOfTheText(t *testing.T) {
	repo, tx := newTx(t)
	q := seedCatalog(t, tx)

	if err := repo.UpsertCard(context.Background(), catalog.Card{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetImageKeys(context.Background(), "sv03-125", "cards/125.webp", "cards/125.thumb.webp", "h"); err != nil {
		t.Fatal(err)
	}

	got := search(t, q, catalog.CardFilter{Query: "e"})

	if !equal(got, []string{"sv03-125", "sv03-010", "30th-001", "30th-TG01"}) {
		t.Errorf("found %v, want Dracaufeu-ex (picture), Ékaïser (begins with e), then Noeunoeuf and Galerie", got)
	}
}

func TestAnEmptyImageKeyIsNoPicture(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	if _, err := tx.Exec(context.Background(), `UPDATE cards SET image_key = '' WHERE id = '30th-024'`); err != nil {
		t.Fatal(err)
	}

	got := search(t, q, catalog.CardFilter{SetIDs: []string{"30th"}})

	if !equal(got, []string{"30th-001", "30th-024", "30th-TG01"}) {
		t.Errorf("found %v: an empty key is no picture, so the order is the catalog's", got)
	}
}

func TestLookupsByIDsAndByNumberKeepTheCatalogOrder(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	byIDs := search(t, q, catalog.CardFilter{IDs: []string{"30th-001", "30th-024", "30th-TG01"}})
	byNumber := search(t, q, catalog.CardFilter{Number: intPtr(24)})

	if !equal(byIDs, []string{"30th-001", "30th-024", "30th-TG01"}) {
		t.Errorf("ids: %v: the collection screens rely on the catalog's own order", byIDs)
	}
	if len(byNumber) < 2 || byNumber[0] != "30th-024" {
		t.Errorf("number: %v", byNumber)
	}
}

func TestPaginationStaysConsistentWithThePictureOrder(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)

	first, more, _ := q.SearchCards(context.Background(), catalog.CardFilter{SetIDs: []string{"30th"}, Limit: 2})
	second, _, _ := q.SearchCards(context.Background(), catalog.CardFilter{SetIDs: []string{"30th"}, Limit: 2, Offset: 2})

	if !more || !equal(ids(first), []string{"30th-024", "30th-001"}) || !equal(ids(second), []string{"30th-TG01"}) {
		t.Errorf("pages = %v then %v (more=%v)", ids(first), ids(second), more)
	}
}

func TestSearchByIllustratorFindsHisCardsWhateverTheCase(t *testing.T) {
	_, tx := newTx(t)
	q := seedCatalog(t, tx)
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `UPDATE cards SET illustrator = 'Satoshi Ito' WHERE id IN ('30th-024', 'sv03-125')`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE cards SET illustrator = 'REND' WHERE id = 'sv03-010'`); err != nil {
		t.Fatal(err)
	}

	got := search(t, q, catalog.CardFilter{Illustrator: "SATOSHI ito", Limit: 50})

	if !sameSet(got, "30th-024", "sv03-125") {
		t.Errorf("cards = %v", got)
	}
	if none := search(t, q, catalog.CardFilter{Illustrator: "Satoshi", Limit: 50}); len(none) != 0 {
		t.Errorf("a part of the name must not match: %v", none)
	}
	if both := search(t, q, catalog.CardFilter{Illustrator: "satoshi ito", Rarities: []string{"Double rare"}, Limit: 50}); !sameSet(both, "sv03-125") {
		t.Errorf("with another filter: %v", both)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_off\`); got != `50\%\_off\\` {
		t.Errorf("escapeLike = %q", got)
	}
}
