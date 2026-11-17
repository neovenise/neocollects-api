package api

import (
	"testing"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

func str(s string) *string { return &s }

func TestImageURLsComeFromOurStorageOnly(t *testing.T) {
	withKeys := catalog.CardRow{ImageKey: str("cards/sv03/125.webp"), ThumbKey: str("cards/sv03/125-thumb.webp")}

	image, thumb := NewImageURLs("https://img.example.com/").Card(withKeys)
	if *image != "https://img.example.com/cards/sv03/125.webp" || *thumb != "https://img.example.com/cards/sv03/125-thumb.webp" {
		t.Errorf("with storage: %q, %q", *image, *thumb)
	}

	if image, thumb = NewImageURLs("").Card(withKeys); image != nil || thumb != nil {
		t.Errorf("without storage there is no URL: %v, %v", image, thumb)
	}
	if image, thumb = NewImageURLs("https://img.example.com").Card(catalog.CardRow{}); image != nil || thumb != nil {
		t.Errorf("a card with no picture must have no URL: %v, %v", image, thumb)
	}
}

func TestObjectNeedsAStorageAndAKey(t *testing.T) {
	if _, ok := NewImageURLs("").Object("a/b.png"); ok {
		t.Error("no storage: no URL")
	}
	if _, ok := NewImageURLs("https://img.example.com").Object(""); ok {
		t.Error("no key: no URL")
	}
	if got, _ := NewImageURLs("https://img.example.com/").Object("/a/b.png"); got != "https://img.example.com/a/b.png" {
		t.Errorf("got %q", got)
	}
}

func TestSetImages(t *testing.T) {
	s := catalog.SetRow{LogoKey: str("sets/a.logo.1234abcd.webp"), SymbolKey: str("sets/a.symbol.1234abcd.webp")}
	u := NewImageURLs("https://img.example.com/")

	if *u.SetLogo(s) != "https://img.example.com/sets/a.logo.1234abcd.webp" || *u.SetSymbol(s) != "https://img.example.com/sets/a.symbol.1234abcd.webp" {
		t.Errorf("logo %q, symbol %q", *u.SetLogo(s), *u.SetSymbol(s))
	}
	if u.SetLogo(catalog.SetRow{}) != nil || u.SetSymbol(catalog.SetRow{}) != nil {
		t.Error("no copy: no URL")
	}
}
