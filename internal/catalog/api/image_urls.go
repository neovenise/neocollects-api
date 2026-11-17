package api

import (
	"strings"

	"github.com/neovenise/neocollects-api/internal/catalog"
)

type ImageURLs struct {
	base string
}

func NewImageURLs(base string) ImageURLs {
	return ImageURLs{base: strings.TrimRight(base, "/")}
}

// Object returns the public URL of a storage key.
func (u ImageURLs) Object(key string) (string, bool) {
	if u.base == "" || key == "" {
		return "", false
	}
	return u.base + "/" + strings.TrimLeft(key, "/"), true
}

// Card returns the URLs of a card's picture and thumbnail.
func (u ImageURLs) Card(c catalog.CardRow) (image, thumb *string) {
	return u.pick(c.ImageKey), u.pick(c.ThumbKey)
}

// SetLogo returns the URL of a set's logo, or nil.
func (u ImageURLs) SetLogo(s catalog.SetRow) *string { return u.pick(s.LogoKey) }

// SetSymbol returns the URL of a set's symbol, or nil.
func (u ImageURLs) SetSymbol(s catalog.SetRow) *string { return u.pick(s.SymbolKey) }

// Foil returns the URLs of a foil's three pictures.
func (u ImageURLs) Foil(f catalog.FoilRow) (front, mask string, etch *string, ok bool) {
	frontURL, mask2 := u.pick(f.FrontKey), u.pick(f.MaskKey)
	if frontURL == nil || mask2 == nil {
		return "", "", nil, false
	}
	return *frontURL, *mask2, u.pick(f.EtchKey), true
}

func (u ImageURLs) pick(key *string) *string {
	if key == nil {
		return nil
	}
	if url, ok := u.Object(*key); ok {
		return &url
	}
	return nil
}
