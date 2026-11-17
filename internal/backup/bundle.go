package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

const (
	dbEntry       = "db.dump"
	manifestEntry = "MANIFEST.json"
	imagesPrefix  = "images/"

	Version = 1
)

type Entry struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`

	HasDatabase bool    `json:"hasDatabase"`
	Objects     int     `json:"objects"`
	Bytes       int64   `json:"bytes"`
	Entries     []Entry `json:"entries"`
}

type Source interface {
	Walk(ctx context.Context, fn func(storage.Object) error) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

type Sink interface {
	Put(ctx context.Context, key string, data []byte) error
}

// Export writes the database dump, every object, then the manifest.
func Export(ctx context.Context, src Source, db io.Reader, dbSize int64, w io.Writer, progress func(done int)) (Manifest, error) {
	tw := tar.NewWriter(w)
	m := Manifest{Version: Version, CreatedAt: time.Now().UTC(), HasDatabase: db != nil}
	if db != nil {
		if err := tw.WriteHeader(&tar.Header{Name: dbEntry, Mode: 0o600, Size: dbSize, Typeflag: tar.TypeReg}); err != nil {
			return m, err
		}
		if n, err := io.Copy(tw, db); err != nil || n != dbSize {
			return m, fmt.Errorf("bundle: the database dump is %d bytes, expected %d (%v)", n, dbSize, err)
		}
	}
	err := src.Walk(ctx, func(o storage.Object) error {
		rc, err := src.Open(ctx, o.Key)
		if err != nil {
			return err
		}
		defer rc.Close()

		if err := tw.WriteHeader(&tar.Header{Name: imagesPrefix + o.Key, Mode: 0o644, Size: o.Size, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		sum := sha256.New()
		n, err := io.Copy(io.MultiWriter(tw, sum), rc)
		if err != nil {
			return fmt.Errorf("bundle: read %s: %w", o.Key, err)
		}
		if n != o.Size {
			return fmt.Errorf("bundle: %s is %d bytes, the listing said %d", o.Key, n, o.Size)
		}
		m.Entries = append(m.Entries, Entry{Key: o.Key, Size: n, SHA256: hex.EncodeToString(sum.Sum(nil))})
		m.Bytes += n
		if progress != nil {
			progress(len(m.Entries))
		}
		return nil
	})
	if err != nil {
		return m, err
	}
	m.Objects = len(m.Entries)
	raw, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	if err := tw.WriteHeader(&tar.Header{Name: manifestEntry, Mode: 0o644, Size: int64(len(raw)), Typeflag: tar.TypeReg}); err != nil {
		return m, err
	}
	if _, err := tw.Write(raw); err != nil {
		return m, err
	}
	return m, tw.Close()
}

const maxObject = 64 << 20

// Import writes the objects to dst, the dump to dbOut.
func Import(ctx context.Context, r io.Reader, dst Sink, dbOut io.Writer, progress func(done int)) (Manifest, error) {
	tr := tar.NewReader(r)
	seen := map[string]Entry{}
	var m Manifest
	gotManifest := false
	for {
		if err := ctx.Err(); err != nil {
			return m, err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return m, fmt.Errorf("bundle: not a readable bundle: %w", err)
		}
		switch {
		case h.Name == dbEntry:
			if dbOut != nil {
				if _, err := io.Copy(dbOut, tr); err != nil {
					return m, fmt.Errorf("bundle: extract the database: %w", err)
				}
			}
		case h.Name == manifestEntry:
			if err := json.NewDecoder(io.LimitReader(tr, 256<<20)).Decode(&m); err != nil {
				return m, fmt.Errorf("bundle: manifest: %w", err)
			}
			gotManifest = true
		case strings.HasPrefix(h.Name, imagesPrefix):
			key := strings.TrimPrefix(h.Name, imagesPrefix)
			if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
				return m, fmt.Errorf("bundle: refusing the object name %q", h.Name)
			}
			if h.Size > maxObject {
				return m, fmt.Errorf("bundle: %s is %d bytes, too big for a picture", key, h.Size)
			}
			data, err := io.ReadAll(io.LimitReader(tr, h.Size))
			if err != nil || int64(len(data)) != h.Size {
				return m, fmt.Errorf("bundle: read %s: %v", key, err)
			}
			sum := sha256.Sum256(data)
			seen[key] = Entry{Key: key, Size: h.Size, SHA256: hex.EncodeToString(sum[:])}
			if err := dst.Put(ctx, key, data); err != nil {
				return m, err
			}
			if progress != nil {
				progress(len(seen))
			}
		default:
			return m, fmt.Errorf("bundle: unexpected entry %q", h.Name)
		}
	}
	if !gotManifest {
		return m, errors.New("bundle: the manifest is missing: the file is incomplete (an export that was cut short?)")
	}
	if m.Version > Version {
		return m, fmt.Errorf("bundle: made by a newer version (%d): update this program", m.Version)
	}
	if len(seen) != m.Objects {
		return m, fmt.Errorf("bundle: %d pictures read, the manifest lists %d", len(seen), m.Objects)
	}
	for _, want := range m.Entries {
		got, ok := seen[want.Key]
		if !ok || got.Size != want.Size || got.SHA256 != want.SHA256 {
			return m, fmt.Errorf("bundle: %s does not match the manifest: the file is damaged", want.Key)
		}
	}
	return m, nil
}
