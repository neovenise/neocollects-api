package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}

// WriteProblem answers with a problem+json error and aborts.
func WriteProblem(c *gin.Context, status int, code, detail string) {
	body, err := json.Marshal(problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	})
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Abort()
	c.Data(status, "application/problem+json", body)
}
