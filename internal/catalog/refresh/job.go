package refresh

import (
	"context"
	"log/slog"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog/importer"
)

const (
	nightlyHour   = 3
	nightlyMinute = 30
)

// Importer imports the catalog.
type Importer interface {
	Run(ctx context.Context, opts importer.Options) (importer.Result, error)
}

// Counts tells whether the catalog is empty.
type Counts interface {
	SetCount(ctx context.Context) (int, error)
}

// Job imports the catalog and its prices every night.
type Job struct {
	importer Importer
	counts   Counts
	logger   *slog.Logger
	now      func() time.Time
}

func New(imp Importer, counts Counts, logger *slog.Logger) *Job {
	return &Job{importer: imp, counts: counts, logger: logger, now: time.Now}
}

// Run imports the catalog every night until ctx ends.
func (j *Job) Run(ctx context.Context) {
	j.startup(ctx)

	for {
		next := NextRun(j.now(), nightlyHour, nightlyMinute)
		j.logger.Info("import: next run", "at", next.Format(time.RFC3339))
		select {
		case <-time.After(time.Until(next)):
		case <-ctx.Done():
			return
		}
		j.runOnce(ctx, DetailsFor(next))
	}
}

// startup fills an empty catalog right away.
func (j *Job) startup(ctx context.Context) {
	n, err := j.counts.SetCount(ctx)
	if err != nil {
		j.logger.Error("import: cannot tell whether the catalog is empty", "error", err)
		return
	}
	if n == 0 {
		j.logger.Info("import: empty catalog, first import")
		j.runOnce(ctx, importer.DetailsAll)
	}
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
