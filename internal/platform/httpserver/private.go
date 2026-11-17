package httpserver

import "github.com/gin-gonic/gin"

// Private forbids any cache from keeping the answer.
func Private(c *gin.Context) { c.Header("Cache-Control", "private, no-store") }
