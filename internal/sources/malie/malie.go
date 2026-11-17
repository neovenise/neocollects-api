package malie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://cdn.malie.io/file/malie-io/tcgl/export/"

const imageHost = "cdn.malie.io"

const maxBody = 16 << 20

type IndexEntry struct {
	Path string `json:"path"`

	Abbr string `json:"abbr"`
}

type Card struct {
	Name            string `json:"name"`
	CollectorNumber struct {
		Numeric *int `json:"numeric"`
	} `json:"collector_number"`
	Rarity *struct {
		Designation string `json:"designation"`
	} `json:"rarity"`
	Foil *struct {
		Type string `json:"type"`
	} `json:"foil"`
	Ext *struct {
		TCGL *struct {
			CardID string `json:"cardID"`
		} `json:"tcgl"`
	} `json:"ext"`
	Images *struct {
		TCGL *struct {
			PNG *struct {
				Front string `json:"front"`
				Foil  string `json:"foil"`
				Etch  string `json:"etch"`
			} `json:"png"`
			JPG *struct {
				Front string `json:"front"`
			} `json:"jpg"`
		} `json:"tcgl"`
	} `json:"images"`
}

// PreviewImage returns a light JPEG picture of the card.
func (c Card) PreviewImage() (string, bool) {
	if c.Images == nil || c.Images.TCGL == nil {
		return "", false
	}
	if jpg := c.Images.TCGL.JPG; jpg != nil && trusted(jpg.Front) {
		return jpg.Front, true
	}
	if png := c.Images.TCGL.PNG; png != nil && trusted(png.Front) {
		return png.Front, true
	}
	return "", false
}

// IsReverseVariant reports whether this is the reverse-holo copy.
func (c Card) IsReverseVariant() bool {
	return c.Ext != nil && c.Ext.TCGL != nil && strings.HasSuffix(c.Ext.TCGL.CardID, "_ph")
}

// FoilType returns the card's foil type, or "".
func (c Card) FoilType() string {
	if c.Foil == nil {
		return ""
	}
	return c.Foil.Type
}

// RarityDesignation returns the card's rarity code, or "".
func (c Card) RarityDesignation() string {
	if c.Rarity == nil {
		return ""
	}
	return c.Rarity.Designation
}

// FoilImages returns the card's face, foil mask and engraving.
func (c Card) FoilImages() (front, mask, etch string, ok bool) {
	if c.FoilType() == "" || c.Images == nil || c.Images.TCGL == nil || c.Images.TCGL.PNG == nil {
		return "", "", "", false
	}
	png := c.Images.TCGL.PNG
	if !trusted(png.Front) || !trusted(png.Foil) {
		return "", "", "", false
	}
	if png.Etch != "" && !trusted(png.Etch) {
		return "", "", "", false
	}
	return png.Front, png.Foil, png.Etch, true
}

func trusted(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == imageHost
}

type Client struct {
	baseURL  string
	language string
	http     *http.Client
}

func NewClient(baseURL, language string) *Client {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{baseURL: baseURL, language: language, http: &http.Client{Timeout: 60 * time.Second}}
}

// Index lists the sets of the export.
func (c *Client) Index(ctx context.Context) (map[string]IndexEntry, error) {
	var byLanguage map[string]map[string]IndexEntry
	if err := c.get(ctx, "index.json", &byLanguage); err != nil {
		return nil, err
	}
	entries, ok := byLanguage[c.language]
	if !ok {
		return nil, fmt.Errorf("malie: no %q in the index", c.language)
	}
	return entries, nil
}

// Set returns the cards of one set file.
func (c *Client) Set(ctx context.Context, path string) ([]Card, error) {
	var cards []Card
	if err := c.get(ctx, path, &cards); err != nil {
		return nil, err
	}
	return cards, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {

	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.Contains(path, "://") {
		return fmt.Errorf("malie: refusing the path %q", path)
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		retry, err := c.once(ctx, c.baseURL+path, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retry {
			return err
		}
	}
	return fmt.Errorf("malie: giving up on %s: %w", path, lastErr)
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
		return true, fmt.Errorf("malie: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, errors.New("malie: not found")
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return true, fmt.Errorf("malie: status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return false, fmt.Errorf("malie: unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		return false, fmt.Errorf("malie: decode response: %w", err)
	}
	return false, nil
}

// ImageURLs returns every picture address of the card.
func (c Card) ImageURLs() []string {
	if c.Images == nil || c.Images.TCGL == nil {
		return nil
	}
	var urls []string
	add := func(u string) {
		if u != "" && trusted(u) {
			urls = append(urls, u)
		}
	}
	if png := c.Images.TCGL.PNG; png != nil {
		add(png.Front)
		add(png.Foil)
		add(png.Etch)
	}
	if jpg := c.Images.TCGL.JPG; jpg != nil {
		add(jpg.Front)
	}
	return urls
}

type Source interface {
	Index(ctx context.Context) (map[string]IndexEntry, error)
	Set(ctx context.Context, path string) ([]Card, error)
}
