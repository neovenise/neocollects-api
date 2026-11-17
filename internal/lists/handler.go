package lists

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/auth"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const (
	maxBody       = 2 << 10
	maxNameLength = 60
)

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// List returns the user's lists.
func (h *Handler) List(c *gin.Context) {
	lists, err := h.repo.Lists(c.Request.Context(), userID(c), c.Query("include") == "cards")
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, gin.H{"items": lists})
}

// Create adds a list.
func (h *Handler) Create(c *gin.Context) {
	name, ok := h.readName(c)
	if !ok {
		return
	}
	list, err := h.repo.Create(c.Request.Context(), userID(c), name)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusCreated, list)
}

// Rename changes a list's name.
func (h *Handler) Rename(c *gin.Context) {
	listID := c.Param("listID")
	if !httpserver.IsUUID(listID) {
		h.fail(c, ErrListNotFound)
		return
	}
	name, ok := h.readName(c)
	if !ok {
		return
	}
	list, err := h.repo.Rename(c.Request.Context(), userID(c), listID, name)
	if err != nil {
		h.fail(c, err)
		return
	}
	httpserver.Private(c)
	c.JSON(http.StatusOK, list)
}

// Remove deletes a list.
func (h *Handler) Remove(c *gin.Context) {
	listID := c.Param("listID")
	if !httpserver.IsUUID(listID) {
		h.fail(c, ErrListNotFound)
		return
	}
	if err := h.repo.Delete(c.Request.Context(), userID(c), listID); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// PutCard adds a card to a list.
func (h *Handler) PutCard(c *gin.Context) {
	listID := c.Param("listID")
	if !httpserver.IsUUID(listID) {
		h.fail(c, ErrListNotFound)
		return
	}
	quantity, ok := readQuantity(c)
	if !ok {
		return
	}
	if err := h.repo.PutCard(c.Request.Context(), userID(c), listID, c.Param("cardID"), quantity); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func readQuantity(c *gin.Context) (*int, bool) {
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxBody))
	if err != nil {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_body", "the body is too large")
		return nil, false
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, true
	}
	var req struct {
		Quantity *int `json:"quantity"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_body", "the body must be a JSON object")
		return nil, false
	}
	if req.Quantity != nil && (*req.Quantity < 1 || *req.Quantity > MaxCardQuantity) {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "invalid_quantity", "quantity must be between 1 and "+strconv.Itoa(MaxCardQuantity))
		return nil, false
	}
	return req.Quantity, true
}

// RemoveCard takes a card out of a list.
func (h *Handler) RemoveCard(c *gin.Context) {
	listID := c.Param("listID")
	if !httpserver.IsUUID(listID) {
		h.fail(c, ErrListNotFound)
		return
	}
	if err := h.repo.RemoveCard(c.Request.Context(), userID(c), listID, c.Param("cardID")); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) readName(c *gin.Context) (string, bool) {
	var req struct {
		Name string `json:"name"`
	}
	if !httpserver.ReadJSON(c, &req, maxBody) {
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "invalid_name", "the name must be 1 to 60 characters, without control characters")
		return "", false
	}
	return name, true
}

func userID(c *gin.Context) string { return auth.PrincipalOf(c).UserID }

func (h *Handler) fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrListNotFound):
		httpserver.WriteProblem(c, http.StatusNotFound, "list_not_found", "")
	case errors.Is(err, ErrDefaultList):
		httpserver.WriteProblem(c, http.StatusForbidden, "default_list", "the default lists cannot be renamed or deleted")
	case errors.Is(err, ErrCardNotFound):
		httpserver.WriteProblem(c, http.StatusNotFound, "card_not_found", "no card has this id")
	case errors.Is(err, ErrTooManyLists):
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "too_many_lists", "you cannot have more lists")
	case errors.Is(err, ErrListFull):
		httpserver.WriteProblem(c, http.StatusUnprocessableEntity, "list_full", "this list cannot hold more cards")
	default:
		h.logger.Error("lists: unexpected error", "error", err, "route", c.FullPath())
		httpserver.WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
	}
}
