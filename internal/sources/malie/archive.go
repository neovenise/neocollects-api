package malie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const manifestName = "MANIFEST.json"

type Manifest struct {
	CreatedAt time.Time `json:"createdAt"`
	Language  string    `json:"language"`
	Sets      int       `json:"sets"`
	Cards     int       `json:"cards"`
	Pictures  int       `json:"pictures"`
	Bytes     int64     `json:"bytes"`
}

type Archive struct {
	dir      string
	language string
}

// OpenArchive opens the local archive of one language.
func OpenArchive(dir, language string) *Archive { return &Archive{dir: dir, language: language} }

// Index returns the archived index of sets.
func (a *Archive) Index(context.Context) (map[string]IndexEntry, error) {
	var byLanguage map[string]map[string]IndexEntry
	if err := readJSON(filepath.Join(a.dir, "index.json"), &byLanguage); err != nil {
		return nil, err
	}
	entries, ok := byLanguage[a.language]
	if !ok {
		return nil, fmt.Errorf("malie archive: no %q in the index", a.language)
	}
	return entries, nil
}

// Set returns the cards of an archived set file.
func (a *Archive) Set(_ context.Context, path string) ([]Card, error) {
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.Contains(path, "://") {
		return nil, fmt.Errorf("malie archive: refusing the path %q", path)
	}
	var cards []Card
	return cards, readJSON(filepath.Join(a.dir, path), &cards)
}

// Picture returns the archived copy of a picture, if any.
func (a *Archive) Picture(rawURL string) ([]byte, bool) {
	path, ok := a.picturePath(rawURL)
	if !ok {
		return nil, false
	}
	data, err := os.ReadFile(path)
	return data, err == nil
}

func (a *Archive) picturePath(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != imageHost || strings.Contains(u.Path, "..") {
		return "", false
	}
	return filepath.Join(a.dir, filepath.FromSlash(strings.TrimPrefix(u.Path, "/"))), true
}

// Save writes a downloaded picture into the archive, atomically.
func (a *Archive) Save(rawURL string, data []byte) {
	path, ok := a.picturePath(rawURL)
	if !ok || len(data) == 0 {
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = writeAtomic(path, data)
}

// Manifest describes a finished archive.
func (a *Archive) Manifest() (Manifest, error) {
	var m Manifest
	return m, readJSON(filepath.Join(a.dir, manifestName), &m)
}

func readJSON(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("malie archive: %w", err)
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)
}

// Verify counts the files the archive's sets miss.
func (a *Archive) Verify(ctx context.Context) (checked int, missing []string, err error) {
	index, err := a.Index(ctx)
	if err != nil {
		return 0, nil, err
	}
	for _, e := range index {
		cards, err := a.Set(ctx, e.Path)
		if err != nil {
			missing = append(missing, e.Path)
			continue
		}
		for _, card := range cards {
			for _, u := range card.ImageURLs() {
				checked++
				path, _ := a.picturePath(u)
				if info, err := os.Stat(path); err != nil || info.Size() == 0 {
					missing = append(missing, u)
				}
			}
		}
	}
	sort.Strings(missing)
	return checked, missing, nil
}
