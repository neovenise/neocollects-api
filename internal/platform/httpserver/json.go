package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// WriteCachedJSON answers JSON with an ETag, or 304 when unchanged.
func WriteCachedJSON(c *gin.Context, v any, maxAge time.Duration) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(int(maxAge.Seconds())))
	if ETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ETagMatches reports whether If-None-Match lists the ETag.
func ETagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == etag || candidate == "*" {
			return true
		}
	}
	return false
}
