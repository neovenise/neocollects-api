package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const snapshotTTL = 10 * time.Minute

type snapshotSetJSON struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	BlockName     string `json:"blockName"`
	OfficialCount int    `json:"officialCount"`
	TotalCount    int    `json:"totalCount"`
}

type snapshotJSON struct {
	Sets     []snapshotSetJSON `json:"sets"`
	Rarities []string          `json:"rarities"`
	Cards    [][5]any          `json:"cards"`
}

type builtSnapshot struct {
	plain, gzipped []byte
	etag           string
	builtAt        time.Time
}

type snapshotCache struct {
	mu    sync.Mutex
	built *builtSnapshot
}

func buildSnapshot(sets []catalog.SnapshotSet, cards []catalog.SnapshotCard, now time.Time) (*builtSnapshot, error) {
	out := snapshotJSON{Sets: make([]snapshotSetJSON, 0, len(sets)), Rarities: []string{}, Cards: make([][5]any, 0, len(cards))}
	setIndex := make(map[string]int, len(sets))
	for i, s := range sets {
		setIndex[s.ID] = i
		out.Sets = append(out.Sets, snapshotSetJSON{ID: s.ID, Code: s.Code, Name: s.Name, BlockName: s.BlockName,
			OfficialCount: s.OfficialCount, TotalCount: s.TotalCount})
	}
	rarityIndex := map[string]int{}
	for _, c := range cards {
		idx, ok := setIndex[c.SetID]
		if !ok {
			continue
		}
		r, seen := rarityIndex[c.Rarity]
		if !seen {
			r = len(out.Rarities)
			rarityIndex[c.Rarity] = r
			out.Rarities = append(out.Rarities, c.Rarity)
		}
		out.Cards = append(out.Cards, [5]any{c.ID, idx, c.LocalID, c.Name, r})
	}

	plain, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(plain); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(plain)
	return &builtSnapshot{plain: plain, gzipped: zipped.Bytes(), etag: `"` + hex.EncodeToString(sum[:16]) + `"`, builtAt: now}, nil
}

// Snapshot returns every set and card for offline use.
func (h *Handler) Snapshot(c *gin.Context) {
	built, err := h.currentSnapshot(c)
	if err != nil {
		h.fail(c, err)
		return
	}

	c.Header("ETag", built.etag)
	c.Header("Cache-Control", "public, max-age=600")
	c.Header("Vary", "Accept-Encoding")
	if httpserver.ETagMatches(c.GetHeader("If-None-Match"), built.etag) {
		c.Status(http.StatusNotModified)
		return
	}
	if acceptsGzip(c.GetHeader("Accept-Encoding")) {
		c.Header("Content-Encoding", "gzip")
		c.Data(http.StatusOK, "application/json; charset=utf-8", built.gzipped)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", built.plain)
}

func (h *Handler) currentSnapshot(c *gin.Context) (*builtSnapshot, error) {
	h.snapshots.mu.Lock()
	defer h.snapshots.mu.Unlock()

	if b := h.snapshots.built; b != nil && h.now().Sub(b.builtAt) < snapshotTTL {
		return b, nil
	}
	sets, cards, err := h.reader.Snapshot(c.Request.Context())
	if err != nil {
		return nil, err
	}
	built, err := buildSnapshot(sets, cards, h.now())
	if err != nil {
		return nil, err
	}
	h.snapshots.built = built
	return built, nil
}

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		coding := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.EqualFold(coding, "gzip") && !strings.Contains(part, "q=0") {
			return true
		}
	}
	return false
}
