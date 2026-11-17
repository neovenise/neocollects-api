package refresh

import (
	"context"
	"log/slog"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog/foils"
	"github.com/neovenise/neocollects-api/internal/catalog/importer"
	"github.com/neovenise/neocollects-api/internal/catalog/mirror"
)

const (
	nightlyHour   = 3
	nightlyMinute = 30
)

type Job struct {
	importer  *importer.Importer
	store     importer.Store
	logger    *slog.Logger
	now       func() time.Time
	foils     FoilRunner
	foilCount func(ctx context.Context) (int, error)
	mirror    MirrorRunner
}

type MirrorRunner interface {
	Run(ctx context.Context, concurrency int) (mirror.Result, error)
}

func (j *Job) WithMirror(runner MirrorRunner) *Job {
	j.mirror = runner
	return j
}

type FoilRunner interface {
	Run(ctx context.Context) (foils.Result, error)
}

func (j *Job) WithFoils(runner FoilRunner, count func(ctx context.Context) (int, error)) *Job {
	j.foils = runner
	j.foilCount = count
	return j
}

func New(imp *importer.Importer, store importer.Store, logger *slog.Logger) *Job {
	return &Job{importer: imp, store: store, logger: logger, now: time.Now}
}

// Run refreshes the catalog every night until ctx ends.
func (j *Job) Run(ctx context.Context) {
	// Fill an empty catalog right away.
	if n, err := j.store.SetCount(ctx); err != nil {
		j.logger.Error("import: cannot tell whether the catalog is empty", "error", err)
	} else if n == 0 {
		j.logger.Info("import: empty catalog, first import")
		j.runOnce(ctx, importer.DetailsAll)
	}
	if j.foils != nil && j.foilCount != nil {
		if n, err := j.foilCount(ctx); err == nil && n == 0 {
			j.logger.Info("import: no foils yet, first foil import")
			j.runFoils(ctx)
		}
	}

	j.runMirror(ctx)

	// Then refresh every night at the scheduled time.
	for {
		next := NextRun(j.now(), nightlyHour, nightlyMinute)
		j.logger.Info("import: next run", "at", next.Format(time.RFC3339))
		select {
		case <-time.After(time.Until(next)):
		case <-ctx.Done():
			return
		}
		details := DetailsFor(next)
		j.runOnce(ctx, details)
		if details == importer.DetailsAll {
			j.runFoils(ctx)
		}
		j.runMirror(ctx)
	}
}

// runMirror copies new or changed pictures to storage.
func (j *Job) runMirror(ctx context.Context) {
	if j.mirror == nil {
		return
	}
	start := time.Now()
	res, err := j.mirror.Run(ctx, 4)
	if err != nil {
		if ctx.Err() == nil {
			j.logger.Error("import: picture copy failed", "error", err)
		}
		return
	}
	j.logger.Info("import: picture copy finished", "duration", time.Since(start).Round(time.Second).String(),
		"copied", res.Copied, "up_to_date", res.UpToDate, "failed", len(res.Failed))
}

// runFoils imports every foil.
func (j *Job) runFoils(ctx context.Context) {
	if j.foils == nil {
		return
	}
	start := time.Now()
	res, err := j.foils.Run(ctx)
	if err != nil {
		j.logger.Error("import: foil run failed", "error", err)
		return
	}
	j.logger.Info("import: foil run finished", "duration", time.Since(start).Round(time.Second).String(),
		"sets", res.Sets, "foils", res.Foils, "unmatched", res.Unmatched, "failed_sets", len(res.FailedSets))
}

// runOnce imports the catalog and logs the outcome.
func (j *Job) runOnce(ctx context.Context, details importer.Details) {
	start := time.Now()
	res, err := j.importer.Run(ctx, importer.Options{Details: details})
	if err != nil {
		j.logger.Error("import: run failed", "error", err)
		return
	}
	j.logger.Info("import: run finished",
		"duration", time.Since(start).Round(time.Second).String(),
		"sets", res.Sets, "cards", res.Cards, "details", res.Details, "prices", res.Prices,
		"failed_sets", len(res.FailedSets), "failed_cards", len(res.FailedCards))
}

// NextRun returns the next occurrence of hour:minute after now.
func NextRun(now time.Time, hour, minute int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, hour, minute, 0, 0, now.Location())
	}
	return next
}

// DetailsFor: everything on Sundays, only new cards otherwise.
func DetailsFor(t time.Time) importer.Details {
	if t.Weekday() == time.Sunday {
		return importer.DetailsAll
	}
	return importer.DetailsNew
}
