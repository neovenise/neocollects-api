package mirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

type FolderStore interface {
	CardImageHash(ctx context.Context, cardID string) (hash string, exists bool, err error)
	SetImageKeys(ctx context.Context, cardID, imageKey, thumbKey, hash string) error
}

type FolderResult struct {
	Copied   int
	UpToDate int

	Unknown []string
	Failed  []string
}

type Folder struct {
	db     FolderStore
	store  Storage
	logger *slog.Logger
}

func NewFolder(db FolderStore, store Storage, logger *slog.Logger) *Folder {
	return &Folder{db: db, store: store, logger: logger}
}

// Run copies every picture of a local folder.
func (f *Folder) Run(ctx context.Context, dir string) (FolderResult, error) {
	var res FolderResult
	entries, err := os.ReadDir(dir)
	if err != nil {
		return res, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".png", ".jpg", ".jpeg", ".webp":
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		cardID := strings.TrimSuffix(name, filepath.Ext(name))
		outcome, err := f.copyFile(ctx, cardID, filepath.Join(dir, name))
		switch {
		case err != nil:
			f.logger.Warn("could not copy a picture from the folder", "file", name, "error", err)
			res.Failed = append(res.Failed, name)
		case outcome == "unknown":
			res.Unknown = append(res.Unknown, name)
		case outcome == "upToDate":
			res.UpToDate++
		default:
			res.Copied++
		}
	}
	return res, nil
}

func (f *Folder) copyFile(ctx context.Context, cardID, path string) (string, error) {
	recorded, exists, err := f.db.CardImageHash(ctx, cardID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "unknown", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	hash := "file-" + hex.EncodeToString(sum[:])[:24]
	if recorded == hash {
		return "upToDate", nil
	}
	decoded, err := storage.Decode(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	cardFile := data
	if decoded.Format != "webp" {
		if cardFile, err = storage.EncodeCard(decoded.Image); err != nil {
			return "", err
		}
	}
	thumbFile, err := storage.EncodeThumb(decoded.Image)
	if err != nil {
		return "", err
	}
	imageKey, thumbKey := ObjectKeys(cardID, hash[len("file-"):])
	if err := f.store.Put(ctx, imageKey, cardFile); err != nil {
		return "", err
	}
	if err := f.store.Put(ctx, thumbKey, thumbFile); err != nil {
		return "", err
	}
	return "copied", f.db.SetImageKeys(ctx, cardID, imageKey, thumbKey, hash)
}
