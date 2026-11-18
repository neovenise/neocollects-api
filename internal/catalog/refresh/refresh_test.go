package refresh

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/neovenise/neocollects-api/internal/catalog/importer"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type fakeImporter struct {
	runs []importer.Options
	err  error
}

func (f *fakeImporter) Run(_ context.Context, opts importer.Options) (importer.Result, error) {
	f.runs = append(f.runs, opts)
	return importer.Result{Sets: 1}, f.err
}

type fakeCounts struct {
	sets int
	err  error
}

func (f fakeCounts) SetCount(context.Context) (int, error) { return f.sets, f.err }

func TestStartupImportsEverythingIntoAnEmptyCatalog(t *testing.T) {
	imp := &fakeImporter{}
	New(imp, fakeCounts{}, testLogger()).startup(context.Background())
	if len(imp.runs) != 1 || imp.runs[0].Details != importer.DetailsAll {
		t.Fatalf("runs = %+v, want one full import", imp.runs)
	}
}

func TestStartupLeavesAFilledCatalogAlone(t *testing.T) {
	imp := &fakeImporter{}
	New(imp, fakeCounts{sets: 100}, testLogger()).startup(context.Background())
	if len(imp.runs) != 0 {
		t.Fatalf("runs = %+v, want none", imp.runs)
	}
}

func TestStartupImportsNothingWhenTheCatalogCannotBeCounted(t *testing.T) {
	imp := &fakeImporter{}
	New(imp, fakeCounts{err: errors.New("db down")}, testLogger()).startup(context.Background())
	if len(imp.runs) != 0 {
		t.Fatalf("runs = %+v, want none on an unreadable catalog", imp.runs)
	}
}

func TestAFailedImportIsLoggedNotFatal(t *testing.T) {
	imp := &fakeImporter{err: errors.New("tcgdex is down")}
	New(imp, fakeCounts{}, testLogger()).runOnce(context.Background(), importer.DetailsNew)
	if len(imp.runs) != 1 {
		t.Fatalf("runs = %+v", imp.runs)
	}
}

func TestDetailsForAskEverythingOnSundaysAndNewCardsOtherDays(t *testing.T) {
	sunday := time.Date(2026, 11, 22, 3, 30, 0, 0, time.UTC)
	if DetailsFor(sunday) != importer.DetailsAll || DetailsFor(sunday.AddDate(0, 0, 1)) != importer.DetailsNew {
		t.Fatal("Sunday must refresh every card, other days only the new ones")
	}
}

func TestNextRunIsTodayBeforeTheHourAndTomorrowAfter(t *testing.T) {
	loc := time.UTC
	cases := []struct{ now, want time.Time }{
		{time.Date(2026, 11, 18, 1, 0, 0, 0, loc), time.Date(2026, 11, 18, 3, 30, 0, 0, loc)},
		{time.Date(2026, 11, 18, 3, 30, 0, 0, loc), time.Date(2026, 11, 19, 3, 30, 0, 0, loc)},
		{time.Date(2026, 11, 30, 23, 0, 0, 0, loc), time.Date(2026, 12, 1, 3, 30, 0, 0, loc)},
	}
	for _, c := range cases {
		if got := NextRun(c.now, 3, 30); !got.Equal(c.want) {
			t.Errorf("NextRun(%v) = %v, want %v", c.now, got, c.want)
		}
	}
}
