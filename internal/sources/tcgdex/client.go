package tcgdex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.tcgdex.net/v2"

const maxBody = 8 << 20

var ErrNotFound = errors.New("tcgdex: not found")

type Client struct {
	baseURL string
	lang    string
	http    *http.Client

	retries int

	backoff time.Duration
}

func NewClient(baseURL, lang string) *Client {
	return &Client{
		baseURL: baseURL,
		lang:    lang,
		http:    &http.Client{Timeout: 30 * time.Second},
		retries: 3,
		backoff: 500 * time.Millisecond,
	}
}

// Series lists the blocks, oldest first.
func (c *Client) Series(ctx context.Context) ([]Serie, error) {
	var series []Serie
	if err := c.get(ctx, "/series", &series); err != nil {
		return nil, err
	}
	return series, nil
}

// SetRefs lists the ids of every set.
func (c *Client) SetRefs(ctx context.Context) ([]SetRef, error) {
	var sets []SetRef
	if err := c.get(ctx, "/sets", &sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// Set returns one set with its card list.
func (c *Client) Set(ctx context.Context, id string) (Set, error) {
	var set Set
	err := c.get(ctx, "/sets/"+url.PathEscape(id), &set)
	return set, err
}

// Card returns one card with its rarity and price.
func (c *Client) Card(ctx context.Context, id string) (Card, error) {
	var card Card
	err := c.get(ctx, "/cards/"+url.PathEscape(id), &card)
	return card, err
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	endpoint := c.baseURL + "/" + url.PathEscape(c.lang) + path

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			wait := c.backoff << (attempt - 1)
			if retryAfter, ok := lastRetryAfter(lastErr); ok {
				wait = retryAfter
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		retry, err := c.once(ctx, endpoint, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
	}
	return fmt.Errorf("tcgdex: giving up on %s: %w", path, lastErr)
}

type retryError struct {
	status     int
	retryAfter time.Duration
}

func (e retryError) Error() string { return fmt.Sprintf("tcgdex: status %d", e.status) }

func lastRetryAfter(err error) (time.Duration, bool) {
	var re retryError
	if errors.As(err, &re) && re.retryAfter > 0 {
		return re.retryAfter, true
	}
	return 0, false
}

func (c *Client) once(ctx context.Context, endpoint string, out any) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")

	req.Header.Set("User-Agent", "neocollects-api")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return true, fmt.Errorf("tcgdex: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		re := retryError{status: resp.StatusCode}
		if seconds, convErr := strconv.Atoi(resp.Header.Get("Retry-After")); convErr == nil && seconds > 0 && seconds <= 60 {
			re.retryAfter = time.Duration(seconds) * time.Second
		}
		return true, re
	case resp.StatusCode != http.StatusOK:
		return false, fmt.Errorf("tcgdex: unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		return false, fmt.Errorf("tcgdex: decode response: %w", err)
	}
	return false, nil
}
