package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neovenise/neocollects-api/internal/platform/httpserver"
)

const (
	maxIDs       = 100
	maxTextParam = 100

	maxFilterValues = 30
)

func optionalInt(c *gin.Context, name string) (*int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", name+" must be a non-negative integer")
		return nil, false
	}
	return &n, true
}

func optionalDate(c *gin.Context, name string) (*string, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	if _, err := time.Parse(time.DateOnly, raw); err != nil {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", name+" must be a date like 2026-11-15")
		return nil, false
	}
	return &raw, true
}

func valuesParam(c *gin.Context, name string) ([]string, bool) {
	values := c.QueryArray(name)
	if len(values) > maxFilterValues {
		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", name+" has too many values")
		return nil, false
	}
	for _, v := range values {
		if v == "" || len(v) > maxTextParam {
			httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", name+" must be 1 to 100 characters")
			return nil, false
		}
	}
	return values, true
}

func idList(c *gin.Context) ([]string, bool) {
	raw := c.Query("ids")
	if raw == "" {
		return nil, true
	}
	var ids []string
	for _, id := range strings.Split(raw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > maxIDs {
		httpserver.WriteProblem(c, http.StatusBadRequest, "too_many_ids", "at most "+strconv.Itoa(maxIDs)+" ids")
		return nil, false
	}
	if len(ids) == 0 {

		httpserver.WriteProblem(c, http.StatusBadRequest, "invalid_parameter", "ids is empty")
		return nil, false
	}
	return ids, true
}
