package collection

import (
	"crypto/sha256"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/auth"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const (
	maxBody = 4 << 10
)

var conditions = map[string]bool{
	"mint": true, "nearMint": true, "excellent": true, "good": true, "lightPlayed": true, "played": true, "poor": true,
}

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// List returns a page of the user's entries.
func (h *Handler) List(c *gin.Context) {
	limit, ok := httpserver.LimitParam(c, httpserver.DefaultPageSize, httpserver.MaxPageSize)
	if !ok {
		return
	}
	offset, ok := httpserver.OffsetParam(c)
	if !ok {
		return
	}
	entries, hasMore, err := h.repo.List(c.Request.Context(), userID(c), c.Query("setId"), limit, offset)
	if err != nil {
		h.fail(c, err)
		return
	}
	page := struct {
		Items      []Entry `json:"items"`
		NextCursor string  `json:"nextCursor,omitempty"`
	}{Items: entries}
	if hasMore {
		page.NextCursor = httpserver.EncodeOffsetCursor(offset + limit)
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, page)
}

// Add puts copies of a card in the collection.
func (h *Handler) Add(c *gin.Context) {
	var req struct {
		CardID    string `json:"cardId"`
		Condition string `json:"condition"`
		Language  string `json:"language"`
		Quantity  *int   `json:"quantity"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	in := NewEntry{CardID: strings.TrimSpace(req.CardID), Condition: req.Condition, Language: strings.ToUpper(strings.TrimSpace(req.Language)), Quantity: 1}
	if req.Quantity != nil {
		in.Quantity = *req.Quantity
	}
	if in.CardID == "" {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "unknown_card", "cardId is required")
		return
	}
	if !validCondition(c, in.Condition) || !validLanguage(c, in.Language) || !validQuantity(c, in.Quantity) {
		return
	}

	var idem *Idempotency
	if key := c.GetHeader("Idempotency-Key"); key != "" {
		if !validKey(key) {
			httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_idempotency_key", "the key must be 1 to 128 printable ASCII characters")
			return
		}

		sum := sha256.Sum256([]byte(strings.Join([]string{in.CardID, in.Condition, in.Language, strconv.Itoa(in.Quantity)}, "\x00")))
		idem = &Idempotency{Key: key, RequestHash: sum[:]}
	}

	result, err := h.repo.Add(c.Request.Context(), userID(c), in, idem)
	if err != nil {
		h.fail(c, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	if result.Replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	httpserver.Private(c)
	c.JSON(status, result.Entry)
}

// Update changes an entry's quantity, condition or language.
func (h *Handler) Update(c *gin.Context) {
	entryID := c.Param("entryID")
	if !httpserver.IsUUID(entryID) {
		h.fail(c, ErrEntryNotFound)
		return
	}
	var req struct {
		Quantity  *int    `json:"quantity"`
		Condition *string `json:"condition"`
		Language  *string `json:"language"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return
	}
	if req.Quantity == nil && req.Condition == nil && req.Language == nil {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_body", "give at least one of quantity, condition, language")
		return
	}
	patch := Patch{Quantity: req.Quantity, Condition: req.Condition}
	if req.Language != nil {
		language := strings.ToUpper(strings.TrimSpace(*req.Language))
		patch.Language = &language
		if !validLanguage(c, language) {
			return
		}
	}
	if req.Quantity != nil && !validQuantity(c, *req.Quantity) {
		return
	}
	if req.Condition != nil && !validCondition(c, *req.Condition) {
		return
	}

	entry, err := h.repo.Update(c.Request.Context(), userID(c), entryID, patch)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, entry)
}

// Remove deletes an entry.
func (h *Handler) Remove(c *gin.Context) {
	entryID := c.Param("entryID")
	if !httpserver.IsUUID(entryID) {
		h.fail(c, ErrEntryNotFound)
		return
	}
	if err := h.repo.Delete(c.Request.Context(), userID(c), entryID); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Summary returns the collection's totals at today's prices.
func (h *Handler) Summary(c *gin.Context) {
	s, err := h.repo.Summary(c.Request.Context(), userID(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, s)
}

func userID(c *gin.Context) string { return auth.PrincipalOf(c).UserID }

func validCondition(c *gin.Context, s string) bool {
	if !conditions[s] {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "invalid_condition", "condition must be mint, nearMint, excellent, good, lightPlayed, played or poor")
		return false
	}
	return true
}

func validLanguage(c *gin.Context, s string) bool {
	if len(s) != 2 || s[0] < 'A' || s[0] > 'Z' || s[1] < 'A' || s[1] > 'Z' {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "invalid_language", "language must be two letters, like FR")
		return false
	}
	return true
}

func validQuantity(c *gin.Context, n int) bool {
	if n < 1 || n > MaxQuantity {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "invalid_quantity", "quantity must be between 1 and "+strconv.Itoa(MaxQuantity))
		return false
	}
	return true
}

func validKey(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func (h *Handler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrCardNotFound):
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "unknown_card", "no card has this id")
	case errors.Is(err, ErrEntryNotFound):
		httpserver.WriteProblem(c, http.StatusNotFound, "entry_not_found", "")
	case errors.Is(err, ErrQuantityTooLarge):
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "quantity_too_large", "the total would go over "+strconv.Itoa(MaxQuantity))
	case errors.Is(err, ErrKeyReused):
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "idempotency_key_reused", "this key was used for a different request")
	default:
		h.logger.Error("collection: unexpected error", "error", err, "route", c.FullPath())
		httpserver.WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
	}
}
