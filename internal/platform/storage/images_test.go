package storage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func card(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: uint8((x ^ y) & 0xFF), A: 255})
		}
	}
	return img
}

func asPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDecodeKnowsJPEGPNGAndWebP(t *testing.T) {
	src := card(120, 160)
	var j bytes.Buffer
	_ = jpeg.Encode(&j, src, nil)
	webpData, err := EncodeCard(src)
	if err != nil {
		t.Fatal(err)
	}

	for name, tt := range map[string]struct {
		data   []byte
		format string
	}{"jpeg": {j.Bytes(), "jpeg"}, "png": {asPNG(t, src), "png"}, "webp": {webpData, "webp"}} {
		got, err := Decode(tt.data)
		if err != nil || got.Format != tt.format || got.Image.Bounds().Dx() != 120 || got.Image.Bounds().Dy() != 160 {
			t.Errorf("%s: format %q, size %v, err %v", name, got.Format, got.Image.Bounds().Size(), err)
		}
	}
}

func TestDecodeRefusesAnythingElse(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<html>not a picture</html>"), {0xFF}} {
		if _, err := Decode(data); err == nil {
			t.Errorf("Decode(%q) must fail", data)
		}
	}
}

func TestTheThumbnailIsScaledDownToItsWidthAndKeepsTheProportions(t *testing.T) {
	data, err := EncodeThumb(card(733, 1024))
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(data)
	if err != nil || got.Format != "webp" {
		t.Fatalf("format %q, err %v", got.Format, err)
	}
	if size := got.Image.Bounds().Size(); size.X != ThumbWidth || size.Y != 419 {
		t.Errorf("thumbnail is %v, want %d x 419 (733 x 1024 scaled)", size, ThumbWidth)
	}
}

func TestAThumbnailIsNeverScaledUp(t *testing.T) {
	data, err := EncodeThumb(card(200, 280))
	if err != nil {
		t.Fatal(err)
	}

	got, _ := Decode(data)
	if size := got.Image.Bounds().Size(); size.X != 200 || size.Y != 280 {
		t.Errorf("thumbnail is %v, a small picture keeps its size", size)
	}
}

func TestACardIsKeptAtItsOwnSize(t *testing.T) {
	src := card(733, 1024)

	data, err := EncodeCard(src)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := Decode(data)
	if got.Image.Bounds().Size() != src.Bounds().Size() {
		t.Errorf("size = %v", got.Image.Bounds().Size())
	}
}

func TestTheFetcherOnlyReadsFromTrustedHostsOverHTTPS(t *testing.T) {
	f := NewFetcher("cdn.malie.io")
	for _, bad := range []string{
		"http://cdn.malie.io/a.png",
		"https://evil.example.com/a.png",
		"https://cdn.malie.io.evil.com/a.png",
		"https://127.0.0.1/a.png",
		"file:///etc/passwd",
		"::not a url",
	} {
		if _, err := f.Fetch(context.Background(), bad); err == nil {
			t.Errorf("Fetch(%q) must be refused", bad)
		}
	}
}

func TestTheFetcherReadsAPictureAndRefusesAnErrorOrAnOversizedFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok.png", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("picture")) })
	mux.HandleFunc("/gone.png", func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
	mux.HandleFunc("/huge.png", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte{1}, maxPicture+10))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")
	f := NewFetcher(host)
	f.http = srv.Client()

	if data, err := f.Fetch(context.Background(), srv.URL+"/ok.png"); err != nil || string(data) != "picture" {
		t.Errorf("ok: %q, %v", data, err)
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/gone.png"); err == nil {
		t.Error("a 404 must be an error")
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/huge.png"); err == nil {
		t.Error("a file over the limit must be refused")
	}
}

func TestLocalCopiesAreUsedBeforeTheNetwork(t *testing.T) {
	asked := ""
	f := NewFetcher("cdn.malie.io").WithLocalCopies(func(u string) ([]byte, bool) {
		asked = u
		parsed, _ := url.Parse(u)
		return []byte("archived " + parsed.Path), strings.HasSuffix(u, "/archived.png")
	})

	data, err := f.Fetch(context.Background(), "https://cdn.malie.io/archived.png")

	if err != nil || string(data) != "archived /archived.png" || asked == "" {
		t.Errorf("data %q, err %v", data, err)
	}

	if _, err := f.Fetch(context.Background(), "https://evil.example.com/other.png"); err == nil {
		t.Error("an address outside the archive and the trusted hosts must still be refused")
	}
}

func TestWhatIsDownloadedIsHandedToTheSaver(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("picture")) }))
	defer srv.Close()
	f := NewFetcher(strings.TrimPrefix(srv.URL, "https://"))
	f.http = srv.Client()
	saved := map[string]string{}
	f.WithSaver(func(u string, data []byte) { saved[u] = string(data) })
	f.WithLocalCopies(func(u string) ([]byte, bool) { return []byte("archived"), strings.HasSuffix(u, "/have.png") })

	_, _ = f.Fetch(context.Background(), srv.URL+"/new.png")
	_, _ = f.Fetch(context.Background(), srv.URL+"/have.png")

	if len(saved) != 1 || saved[srv.URL+"/new.png"] != "picture" {
		t.Errorf("saved = %v", saved)
	}
}
