package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxPicture = 8 << 20

type Fetcher struct {
	hosts map[string]bool
	http  *http.Client
	local func(url string) ([]byte, bool)
	save  func(url string, data []byte)
}

func (f *Fetcher) WithSaver(save func(url string, data []byte)) *Fetcher {
	f.save = save
	return f
}

func (f *Fetcher) WithLocalCopies(local func(url string) ([]byte, bool)) *Fetcher {
	f.local = local
	return f
}

func NewFetcher(hosts ...string) *Fetcher {
	allowed := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		allowed[h] = true
	}
	return &Fetcher{hosts: allowed, http: &http.Client{Timeout: 60 * time.Second}}
}

// Fetch downloads the picture at rawURL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || !f.hosts[u.Host] {
		return nil, fmt.Errorf("images: refusing to fetch %q", rawURL)
	}
	if f.local != nil {
		if data, ok := f.local(rawURL); ok {
			return data, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "neocollects-api")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("images: %s answered %d", u.Host, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPicture+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPicture {
		return nil, fmt.Errorf("images: %s is bigger than %d bytes", rawURL, maxPicture)
	}
	if f.save != nil {
		f.save(rawURL, data)
	}
	return data, nil
}
