package malie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const cardsFolder = "https://cdn.malie.io/file/malie-io/tcgl/cards/png/"

// PictureURLs builds a card's face, mask and engraving addresses.
func PictureURLs(lang, gameSet, number string) (front, foil, etch string) {
	base := fmt.Sprintf("%s%s/%s/%s_%s_%s_std", cardsFolder, lang, gameSet, gameSet, lang, number)
	return base + ".png", base + ".foil.png", base + ".etch.png"
}

// Exists checks a picture is online by fetching one byte.
func (c *Client) Exists(ctx context.Context, rawURL string) (bool, error) {
	if !trusted(rawURL) {
		return false, fmt.Errorf("malie: refusing the address %q", rawURL)
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
		found, retry, err := c.probe(ctx, rawURL)
		if err == nil {
			return found, nil
		}
		lastErr = err
		if !retry {
			return false, err
		}
	}
	return false, fmt.Errorf("malie: giving up on %s: %w", rawURL, lastErr)
}

func (c *Client) probe(ctx context.Context, rawURL string) (found, retry bool, err error) {
	if _, parseErr := url.Parse(rawURL); parseErr != nil {
		return false, false, parseErr
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false, false, err
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "neocollects-api")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return false, false, ctx.Err()
		}
		return false, true, fmt.Errorf("malie: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent:
		return true, false, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, false, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return false, true, fmt.Errorf("malie: status %d", resp.StatusCode)
	default:
		return false, false, fmt.Errorf("malie: unexpected status %d", resp.StatusCode)
	}
}
