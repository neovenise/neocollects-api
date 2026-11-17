package backup

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

type memBucket struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newBucket(objects map[string]string) *memBucket {
	b := &memBucket{objects: map[string][]byte{}}
	for k, v := range objects {
		b.objects[k] = []byte(v)
	}
	return b
}

func (b *memBucket) Walk(_ context.Context, fn func(storage.Object) error) error {
	keys := make([]string, 0, len(b.objects))
	for k := range b.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := fn(storage.Object{Key: k, Size: int64(len(b.objects[k]))}); err != nil {
			return err
		}
	}
	return nil
}

func (b *memBucket) Open(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(b.objects[key])), nil
}

func (b *memBucket) Put(_ context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = append([]byte(nil), data...)
	return nil
}

var sample = map[string]string{
	"cards/sv03-125.1.abc.webp":       "the card",
	"cards/sv03-125.1.abc.thumb.webp": "its thumbnail",
	"foils/sv03-125.1.def.mask.webp":  strings.Repeat("m", 5000),
	"sets/sv03.logo.123.webp":         "",
	"cards/swsh12.5gg-GG56.1.x.webp":  "a gallery card",
}

func export(t *testing.T, objects map[string]string, db string) []byte {
	t.Helper()
	var out bytes.Buffer
	var dbReader io.Reader
	if db != "" {
		dbReader = strings.NewReader(db)
	}
	if _, err := Export(context.Background(), newBucket(objects), dbReader, int64(len(db)), &out, nil); err != nil {
		t.Fatalf("Export: %v", err)
	}
	return out.Bytes()
}

func TestABundleBringsBackEveryPictureAndTheDatabaseDump(t *testing.T) {
	file := export(t, sample, "PGDMP-binary-dump")
	target := newBucket(nil)
	var dump bytes.Buffer

	m, err := Import(context.Background(), bytes.NewReader(file), target, &dump, nil)

	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if m.Objects != len(sample) || !m.HasDatabase || dump.String() != "PGDMP-binary-dump" {
		t.Errorf("manifest = %+v, dump = %q", m, dump.String())
	}
	for key, want := range sample {
		if got := string(target.objects[key]); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if len(target.objects) != len(sample) {
		t.Errorf("%d objects, want %d", len(target.objects), len(sample))
	}
}

func TestABundleWithoutADatabaseIsStillAValidBundle(t *testing.T) {
	file := export(t, map[string]string{"a.webp": "x"}, "")

	m, err := Import(context.Background(), bytes.NewReader(file), newBucket(nil), nil, nil)

	if err != nil || m.HasDatabase || m.Objects != 1 {
		t.Errorf("manifest = %+v, err = %v", m, err)
	}
}

func TestADamagedPictureIsDetectedNotLoadedAsIfFine(t *testing.T) {
	file := export(t, map[string]string{"cards/a.webp": "the picture bytes"}, "")
	damaged := bytes.Replace(file, []byte("the picture bytes"), []byte("the picture bytez"), 1)

	_, err := Import(context.Background(), bytes.NewReader(damaged), newBucket(nil), nil, nil)

	if err == nil || !strings.Contains(err.Error(), "does not match the manifest") {
		t.Errorf("err = %v", err)
	}
}

func TestABundleCutShortHasNoManifestAndIsRefused(t *testing.T) {
	file := export(t, sample, "dump")
	manifestAt := bytes.LastIndex(file, []byte("MANIFEST.json"))

	_, err := Import(context.Background(), bytes.NewReader(file[:manifestAt]), newBucket(nil), nil, nil)

	if err == nil {
		t.Error("a bundle without its manifest must be refused")
	}
}

func TestAnEntryThatEscapesTheBucketIsRefused(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Name: "images/../../etc/passwd", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	target := newBucket(nil)

	_, err := Import(context.Background(), &buf, target, nil, nil)

	if err == nil || len(target.objects) != 0 {
		t.Errorf("err = %v, objects = %v", err, target.objects)
	}
}

func TestAnExportWhoseListingLiesAboutASizeFailsInsteadOfWritingACorruptEntry(t *testing.T) {
	liar := &lyingBucket{memBucket: newBucket(map[string]string{"a.webp": "four"})}

	_, err := Export(context.Background(), liar, nil, 0, io.Discard, nil)

	if err == nil {
		t.Error("a size that does not match what is read must fail the export")
	}
}

type lyingBucket struct{ *memBucket }

func (l *lyingBucket) Walk(_ context.Context, fn func(storage.Object) error) error {
	return fn(storage.Object{Key: "a.webp", Size: 99})
}
