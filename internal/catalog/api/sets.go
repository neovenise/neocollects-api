package api

import (
	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

// Blocks returns every block, newest first.
func (h *Handler) Blocks(c *gin.Context) {
	blocks, err := h.reader.Blocks(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]blockJSON, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, blockJSON{ID: b.ID, Name: b.Name})
	}
	httpserver.WriteCachedJSON(c, listJSON[blockJSON]{Items: out}, cacheAge)
}

// Sets returns every set, grouped by block.
func (h *Handler) Sets(c *gin.Context) {
	officialCount, ok := optionalInt(c, "officialCount")
	if !ok {
		return
	}
	sets, err := h.reader.Sets(c.Request.Context(), catalog.SetFilter{BlockID: c.Query("blockId"), OfficialCount: officialCount})
	if err != nil {
		h.fail(c, err)
		return
	}
	out := make([]setJSON, 0, len(sets))
	for _, s := range sets {
		out = append(out, h.setJSON(s))
	}
	httpserver.WriteCachedJSON(c, listJSON[setJSON]{Items: out}, cacheAge)
}

// Set returns one set.
func (h *Handler) Set(c *gin.Context) {
	s, err := h.reader.Set(c.Request.Context(), c.Param("setId"))
	if err != nil {
		h.notFoundOr(c, err, "set_not_found")
		return
	}
	httpserver.WriteCachedJSON(c, h.setJSON(s), cacheAge)
}

// SetCards returns every card of a set in printed order.
func (h *Handler) SetCards(c *gin.Context) {
	cards, err := h.reader.SetCards(c.Request.Context(), c.Param("setId"))
	if err != nil {
		h.notFoundOr(c, err, "set_not_found")
		return
	}
	httpserver.WriteCachedJSON(c, listJSON[cardSummaryJSON]{Items: h.cardSummaries(cards)}, cacheAge)
}
