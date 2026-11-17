package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/catalog"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const cacheAge = 5 * time.Minute

type Reader interface {
	Blocks(ctx context.Context) ([]catalog.BlockRow, error)
	Sets(ctx context.Context, f catalog.SetFilter) ([]catalog.SetRow, error)
	Set(ctx context.Context, id string) (catalog.SetRow, error)
	SetCards(ctx context.Context, setID string) ([]catalog.CardRow, error)
	Card(ctx context.Context, id string) (catalog.CardDetailRow, error)
	Foil(ctx context.Context, cardID string) (catalog.FoilRow, error)
	PriceHistory(ctx context.Context, cardID string, from, to *string) ([]catalog.PricePointRow, error)
	SearchCards(ctx context.Context, f catalog.CardFilter) ([]catalog.CardRow, bool, error)
	Facets(ctx context.Context) (catalog.Facets, error)
	Snapshot(ctx context.Context) ([]catalog.SnapshotSet, []catalog.SnapshotCard, error)
}

type Handler struct {
	reader    Reader
	urls      ImageURLs
	logger    *slog.Logger
	snapshots snapshotCache

	now func() time.Time
}

func NewHandler(reader Reader, urls ImageURLs, logger *slog.Logger) *Handler {
	return &Handler{reader: reader, urls: urls, logger: logger, now: time.Now}
}

// OnlyFrench rejects any lang other than French.
func OnlyFrench(c *gin.Context) {
	if lang := c.Query("lang"); lang != "" && lang != "fr" {
		httpserver.WriteProblem(c, http.StatusBadRequest, "unsupported_language", `only "fr" is available`)
	}
}

func (h *Handler) notFoundOr(c *gin.Context, err error, code string) {
	if errors.Is(err, catalog.ErrNotFound) {
		httpserver.WriteProblem(c, http.StatusNotFound, code, "")
		return
	}
	h.fail(c, err)
}

func (h *Handler) fail(c *gin.Context, err error) {
	if c.Request.Context().Err() != nil {
		return
	}
	h.logger.Error("catalog request failed", "route", c.FullPath(), "error", err)
	httpserver.WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
}
