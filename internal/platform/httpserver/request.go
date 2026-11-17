package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ReadJSON decodes a small JSON body, or answers 400.
func ReadJSON(c *gin.Context, v any, maxBytes int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	if err := json.NewDecoder(c.Request.Body).Decode(v); err != nil {
		WriteProblem(c, http.StatusBadRequest, "invalid_body", "the body must be a JSON object")
		return false
	}
	return true
}

// IsUUID checks an id's shape before it reaches the database.
func IsUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if s[i] != '-' {
				return false
			}
		case (s[i] >= '0' && s[i] <= '9') || (s[i] >= 'a' && s[i] <= 'f') || (s[i] >= 'A' && s[i] <= 'F'):
		default:
			return false
		}
	}
	return true
}
