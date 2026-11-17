// Command api runs the HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/neovenise/neocollects-api/internal/app"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Read the configuration from the environment.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Bring the database schema up to date.
	if err := database.Migrate(cfg.DatabaseURL, "migrations"); err != nil {
		logger.Error("failed to apply migrations", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Start the nightly catalog refresh in the background.
	if cfg.ImportEnabled {
		job, err := app.CatalogJob(cfg, pool, logger)
		if err != nil {
			logger.Error("image storage is misconfigured", "error", err)
			os.Exit(1)
		}
		go job.Run(ctx)
	}
	if cfg.InviteCode == "" {
		logger.Warn("INVITE_CODE is not set: sign-up is closed")
	}

	// Build the router with every route.
	handler, err := app.NewServer(cfg, pool, logger)
	if err != nil {
		logger.Error("accounts are misconfigured", "error", err)
		os.Exit(1)
	}
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for a stop signal, then drain open requests.
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.Info("shutting down")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
