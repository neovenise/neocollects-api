package workpool

import (
	"context"
	"sync"
)

// ForEach runs fn on every item, at most limit at once, and waits for the started calls.
// It stops starting new ones when ctx ends, then returns ctx.Err().
func ForEach[T any](ctx context.Context, items []T, limit int, fn func(T)) error {
	if limit <= 0 {
		limit = 1
	}
	var (
		wg    sync.WaitGroup
		slots = make(chan struct{}, limit)
	)
	for _, item := range items {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			fn(item)
		}()
	}
	wg.Wait()
	return ctx.Err()
}
