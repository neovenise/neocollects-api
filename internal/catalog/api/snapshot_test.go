package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

func snapshotReader() *fakeReader {
	return &fakeReader{
		snapSets: []catalog.SnapshotSet{
			{ID: "sv03", Code: "OBF", Name: "Flammes Obsidiennes", BlockName: "Écarlate et Violet", OfficialCount: 197, TotalCount: 230},
			{ID: "swshp", Name: "Promos Épée et Bouclier", BlockName: "Épée et Bouclier", OfficialCount: 0, TotalCount: 301},
		},
		snapCards: []catalog.SnapshotCard{
			{ID: "sv03-125", SetID: "sv03", LocalID: "125", Name: "Dracaufeu-ex", Rarity: "Double rare"},
			{ID: "swshp-SWSH001", SetID: "swshp", LocalID: "SWSH001", Name: "Pikachu", Rarity: "Promo"},
			{ID: "sv03-001", SetID: "sv03", LocalID: "001", Name: "Mystherbe", Rarity: "Double rare"},
			{ID: "ghost-1", SetID: "unknown-set", LocalID: "1", Name: "Orpheline", Rarity: "Rare"},
		},
	}
}

func snapshotHandler(r Reader, clock func() time.Time) (http.Handler, *Handler) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(r, NewImageURLs(""), logger)
	h.now = clock
	return mount(httpserver.New(logger), h), h
}

func TestSnapshotIsCompactAndComplete(t *testing.T) {
	h, _ := snapshotHandler(snapshotReader(), time.Now)

	rec := get(t, h, "/v1/catalog/snapshot")

	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("status = %d, encoding = %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
	var snap struct {
		Sets     []map[string]any `json:"sets"`
		Rarities []string         `json:"rarities"`
		Cards    [][]any          `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("body: %v", err)
	}

	if len(snap.Sets) != 2 || snap.Sets[0]["code"] != "OBF" || snap.Sets[0]["officialCount"] != float64(197) {
		t.Errorf("sets = %v", snap.Sets)
	}
	if len(snap.Cards) != 3 {
		t.Fatalf("a card of an unknown set must be left out: %v", snap.Cards)
	}
	first := snap.Cards[0]
	if first[0] != "sv03-125" || first[1] != float64(0) || first[2] != "125" || first[3] != "Dracaufeu-ex" {
		t.Errorf("card = %v", first)
	}
	if snap.Cards[1][2] != "SWSH001" || snap.Cards[1][1] != float64(1) {
		t.Errorf("a promo keeps its lettered number: %v", snap.Cards[1])
	}

	if len(snap.Rarities) != 2 || snap.Cards[0][4] != snap.Cards[2][4] {
		t.Errorf("rarities = %v, indexes %v / %v", snap.Rarities, snap.Cards[0][4], snap.Cards[2][4])
	}
}

func TestSnapshotGzipAndETagCoverBothEncodings(t *testing.T) {
	h, _ := snapshotHandler(snapshotReader(), time.Now)

	plain := get(t, h, "/v1/catalog/snapshot")
	zipped := get(t, h, "/v1/catalog/snapshot", "Accept-Encoding", "gzip, deflate")

	if zipped.Header().Get("Content-Encoding") != "gzip" || zipped.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("headers = %v", zipped.Header())
	}
	reader, err := gzip.NewReader(bytes.NewReader(zipped.Body.Bytes()))
	if err != nil {
		t.Fatalf("not gzip: %v", err)
	}
	body, _ := io.ReadAll(reader)
	if !bytes.Equal(body, plain.Body.Bytes()) {
		t.Error("the compressed body must unpack to the plain one")
	}
	if plain.Header().Get("ETag") == "" || plain.Header().Get("ETag") != zipped.Header().Get("ETag") {
		t.Errorf("one ETag for both encodings: %q / %q", plain.Header().Get("ETag"), zipped.Header().Get("ETag"))
	}

	again := get(t, h, "/v1/catalog/snapshot", "Accept-Encoding", "gzip", "If-None-Match", plain.Header().Get("ETag"))
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 {
		t.Errorf("an up-to-date client must get 304: %d", again.Code)
	}
	if noGzip := get(t, h, "/v1/catalog/snapshot", "Accept-Encoding", "gzip;q=0"); noGzip.Header().Get("Content-Encoding") != "" {
		t.Error("gzip;q=0 means no gzip")
	}
}

func TestSnapshotIsBuiltOnceThenAgainAfterItsTTL(t *testing.T) {
	r := snapshotReader()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	h, _ := snapshotHandler(r, func() time.Time { return now })

	get(t, h, "/v1/catalog/snapshot")
	get(t, h, "/v1/catalog/snapshot", "Accept-Encoding", "gzip")
	if r.snapCalls != 1 {
		t.Errorf("built %d times within the TTL, want 1", r.snapCalls)
	}

	now = now.Add(snapshotTTL + time.Second)
	get(t, h, "/v1/catalog/snapshot")
	if r.snapCalls != 2 {
		t.Errorf("built %d times after the TTL, want 2", r.snapCalls)
	}
}

func TestSnapshotFailureIsAnInternalError(t *testing.T) {
	r := snapshotReader()
	r.err = context.DeadlineExceeded
	h, _ := snapshotHandler(r, time.Now)

	expectProblem(t, get(t, h, "/v1/catalog/snapshot"), http.StatusInternalServerError, "internal_error")
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"gzip": true, "gzip, deflate, br": true, "br, GZIP": true, "gzip;q=0": false, "identity": false, "": false, "deflate": false,
	} {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}
