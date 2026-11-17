package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func New(logger *slog.Logger) *gin.Engine {
	engine := gin.New()

	_ = engine.SetTrustedProxies(nil)
	engine.HandleMethodNotAllowed = true

	engine.Use(requestLog(logger), gin.CustomRecovery(func(c *gin.Context, err any) {
		logger.Error("panic recovered", "error", err, "path", c.FullPath())
		WriteProblem(c, http.StatusInternalServerError, "internal_error", "")
	}))

	engine.NoRoute(func(c *gin.Context) {
		WriteProblem(c, http.StatusNotFound, "route_not_found", "")
	})
	engine.NoMethod(func(c *gin.Context) {
		WriteProblem(c, http.StatusMethodNotAllowed, "method_not_allowed", "")
	})

	return engine
}

// Health answers 200 when the database answers.
func Health(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {

		if err := db.Ping(c.Request.Context()); err != nil {
			WriteProblem(c, http.StatusServiceUnavailable, "database_unavailable", "")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func requestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds())
	}
}
