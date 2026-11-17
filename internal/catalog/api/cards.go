package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

// Card returns one card with its set and price.
func (h *Handler) Card(c *gin.Context) {
	d, err := h.reader.Card(c.Request.Context(), c.Param("cardId"))
	if err != nil {
		h.notFoundOr(c, err, "card_not_found")
		return
	}
	out := cardJSON{cardSummaryJSON: h.cardSummary(d.CardRow), Set: h.setJSON(d.Set), Illustrator: d.Illustrator}
	if d.PriceCents != nil {
		out.PriceCents, out.PriceCurrency = d.PriceCents, d.PriceCurrency
		if d.PriceUpdatedAt != nil {

			utc := d.PriceUpdatedAt.UTC()
			out.PriceUpdatedAt = &utc
		}
		if d.PriceAvg1Cents != nil || d.PriceAvg7Cents != nil || d.PriceAvg30Cents != nil {
			out.PriceAverages = &priceAveragesJSON{Day: d.PriceAvg1Cents, Week: d.PriceAvg7Cents, Month: d.PriceAvg30Cents}
		}
	}
	httpserver.WriteCachedJSON(c, out, cacheAge)
}

// Foil returns the foil look and pictures of a card.
func (h *Handler) Foil(c *gin.Context) {
	f, err := h.reader.Foil(c.Request.Context(), c.Param("cardId"))
	if err != nil {
		h.notFoundOr(c, err, "foil_not_found")
		return
	}
	front, mask, etch, ok := h.urls.Foil(f)
	if !ok {

		httpserver.WriteProblem(c, http.StatusNotFound, "foil_not_found", "")
		return
	}
	out := foilJSON{Look: f.Look, FrontURL: front, MaskURL: mask, EtchURL: etch}
	httpserver.WriteCachedJSON(c, out, cacheAge)
}

// PriceHistory returns a card's price points over a period.
func (h *Handler) PriceHistory(c *gin.Context) {
	from, ok := optionalDate(c, "from")
	if !ok {
		return
	}
	to, ok := optionalDate(c, "to")
	if !ok {
		return
	}
	points, err := h.reader.PriceHistory(c.Request.Context(), c.Param("cardId"), from, to)
	if err != nil {
		h.notFoundOr(c, err, "card_not_found")
		return
	}
	out := make([]pricePointJSON, 0, len(points))
	for _, p := range points {
		out = append(out, pricePointJSON{Date: p.Day, Cents: p.Cents})
	}
	httpserver.WriteCachedJSON(c, listJSON[pricePointJSON]{Items: out}, cacheAge)
}
