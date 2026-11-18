package workpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestForEachRunsEveryItemOnce(t *testing.T) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	var mu sync.Mutex
	seen := map[int]int{}
	if err := ForEach(context.Background(), items, 4, func(i int) {
		mu.Lock()
		seen[i]++
		mu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 100 {
		t.Fatalf("saw %d items, want 100", len(seen))
	}
	for i, n := range seen {
		if n != 1 {
			t.Fatalf("item %d ran %d times", i, n)
		}
	}
}

func TestForEachNeverExceedsTheLimit(t *testing.T) {
	var running, peak atomic.Int32
	items := make([]int, 40)
	_ = ForEach(context.Background(), items, 3, func(int) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		running.Add(-1)
	})
	if peak.Load() > 3 {
		t.Fatalf("peak concurrency %d, limit 3", peak.Load())
	}
}

func TestForEachStopsFeedingWhenCancelledAndWaitsForStartedCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var started, finished atomic.Int32
	items := make([]int, 50)
	err := ForEach(ctx, items, 2, func(int) {
		started.Add(1)
		cancel()
		time.Sleep(5 * time.Millisecond)
		finished.Add(1)
	})
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if started.Load() != finished.Load() {
		t.Fatalf("returned with %d calls still running", started.Load()-finished.Load())
	}
	if started.Load() >= 50 {
		t.Fatalf("kept starting calls after cancellation (%d)", started.Load())
	}
}

func TestForEachTreatsANonPositiveLimitAsOne(t *testing.T) {
	var running, peak atomic.Int32
	_ = ForEach(context.Background(), make([]int, 10), 0, func(int) {
		n := running.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		time.Sleep(time.Millisecond)
		running.Add(-1)
	})
	if peak.Load() != 1 {
		t.Fatalf("peak %d, want 1", peak.Load())
	}
}
