package malie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type CreateOptions struct {
	Concurrency int

	Progress func(done, total int)
}

// Create downloads the whole export and its pictures into dir.
func (c *Client) Create(ctx context.Context, dir string, opts CreateOptions) (Manifest, error) {
	m := Manifest{CreatedAt: time.Now().UTC(), Language: c.language}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 6
	}

	if err := c.download(ctx, c.baseURL+"index.json", filepath.Join(dir, "index.json")); err != nil {
		return m, err
	}

	index, err := OpenArchive(dir, c.language).Index(ctx)
	if err != nil {
		return m, err
	}
	paths := make([]string, 0, len(index))
	for _, e := range index {
		paths = append(paths, e.Path)
	}
	sort.Strings(paths)

	seen := map[string]bool{}
	var pictures []string
	for _, p := range paths {
		if err := c.download(ctx, c.baseURL+p, filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			return m, err
		}
		cards, err := OpenArchive(dir, c.language).Set(ctx, p)
		if err != nil {
			return m, err
		}
		m.Sets++
		m.Cards += len(cards)
		for _, card := range cards {
			for _, u := range card.ImageURLs() {
				if !seen[u] {
					seen[u] = true
					pictures = append(pictures, u)
				}
			}
		}
	}
	sort.Strings(pictures)
	m.Pictures = len(pictures)

	arch := OpenArchive(dir, c.language)
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		slots    = make(chan struct{}, opts.Concurrency)
		done     int
		firstErr error
	)
	for _, u := range pictures {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return m, ctx.Err()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			path, _ := arch.picturePath(u)
			err := c.download(ctx, u, path)

			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil && ctx.Err() == nil {
				firstErr = fmt.Errorf("%s: %w", u, err)
			}
			done++
			if opts.Progress != nil && (done%250 == 0 || done == len(pictures)) {
				opts.Progress(done, len(pictures))
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return m, err
	}
	if firstErr != nil {
		return m, firstErr
	}

	for _, u := range pictures {
		path, _ := arch.picturePath(u)
		if info, err := os.Stat(path); err == nil {
			m.Bytes += info.Size()
		}
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	return m, writeAtomic(filepath.Join(dir, manifestName), out)
}

func (c *Client) download(ctx context.Context, rawURL, path string) error {
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		lastErr = c.downloadOnce(ctx, rawURL, path)
		if lastErr == nil || ctx.Err() != nil {
			return lastErr
		}
	}
	return lastErr
}

func (c *Client) downloadOnce(ctx context.Context, rawURL, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "neocollects-api")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n == 0 {
		err = errors.New("empty answer")
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".partial"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
