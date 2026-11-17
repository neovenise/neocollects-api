package httpserver

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

const offsetCursorPrefix = "offset:"

// EncodeOffsetCursor turns an offset into an opaque cursor.
func EncodeOffsetCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(offsetCursorPrefix + strconv.Itoa(offset)))
}

// LimitParam reads ?limit, or answers 400.
func LimitParam(c *gin.Context, def, max int) (int, bool) {
	raw := c.Query("limit")
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		WriteProblem(c, http.StatusBadRequest, "invalid_parameter", "limit must be a positive integer")
		return 0, false
	}
	return min(n, max), true
}

// OffsetParam reads ?cursor as an offset, or answers 400.
func OffsetParam(c *gin.Context) (int, bool) {
	raw := c.Query("cursor")
	if raw == "" {
		return 0, true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err == nil && strings.HasPrefix(string(decoded), offsetCursorPrefix) {
		if offset, convErr := strconv.Atoi(strings.TrimPrefix(string(decoded), offsetCursorPrefix)); convErr == nil && offset >= 0 {
			return offset, true
		}
	}
	WriteProblem(c, http.StatusBadRequest, "invalid_cursor", "")
	return 0, false
}
