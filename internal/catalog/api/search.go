package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

// SearchCards returns a page of cards matching the filters.
func (h *Handler) SearchCards(c *gin.Context) {
	filter := catalog.CardFilter{Name: c.Query("name"), ExactName: c.Query("exactName"), Query: c.Query("q"), Illustrator: strings.TrimSpace(c.Query("illustrator"))}

	var ok bool
	if filter.Number, ok = optionalInt(c, "number"); !ok {
		return
	}
	if filter.SetSize, ok = optionalInt(c, "setSize"); !ok {
		return
	}
	if filter.IDs, ok = idList(c); !ok {
		return
	}
	if len(filter.Name) > maxTextParam || len(filter.ExactName) > maxTextParam || len(filter.Query) > maxTextParam || len(filter.Illustrator) > maxTextParam {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", "name is too long")
		return
	}

	for _, f := range []struct {
		name string
		into *[]string
	}{
		{"setId", &filter.SetIDs}, {"rarity", &filter.Rarities}, {"category", &filter.Categories}, {"type", &filter.Types},
		{"stage", &filter.Stages}, {"trainerType", &filter.TrainerTypes}, {"regulationMark", &filter.RegulationMarks},
	} {
		if *f.into, ok = valuesParam(c, f.name); !ok {
			return
		}
	}
	if filter.Limit, ok = httpserver.LimitParam(c, httpserver.DefaultPageSize, httpserver.MaxPageSize); !ok {
		return
	}
	if filter.Offset, ok = httpserver.OffsetParam(c); !ok {
		return
	}

	cards, hasMore, err := h.reader.SearchCards(c.Request.Context(), filter)
	if err != nil {
		h.fail(c, err)
		return
	}
	page := pageJSON[cardSummaryJSON]{Items: h.cardSummaries(cards)}
	if hasMore {
		next := httpserver.EncodeOffsetCursor(filter.Offset + filter.Limit)
		page.NextCursor = &next
	}
	httpserver.WriteCachedJSON(c, page, cacheAge)
}

// Facets counts the matching cards by filter value.
func (h *Handler) Facets(c *gin.Context) {
	facets, err := h.reader.Facets(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.WriteCachedJSON(c, facets, cacheAge)
}
