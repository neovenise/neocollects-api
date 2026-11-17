package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/bep/gowebp/libwebp"
	"github.com/bep/gowebp/libwebp/webpoptions"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	CardQuality = 85

	ThumbQuality = 70

	ThumbWidth = 300
)

type Decoded struct {
	Image image.Image

	Format string
}

// Decode reads a JPEG, PNG or WebP picture.
func Decode(data []byte) (Decoded, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
		img, err := jpeg.Decode(bytes.NewReader(data))
		return Decoded{Image: img, Format: "jpeg"}, err
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		img, err := png.Decode(bytes.NewReader(data))
		return Decoded{Image: img, Format: "png"}, err
	case len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		img, err := webp.Decode(bytes.NewReader(data))
		return Decoded{Image: img, Format: "webp"}, err
	}
	return Decoded{}, errors.New("images: not a JPEG, PNG or WebP picture")
}

// EncodeCard encodes a picture as lossy WebP at full size.
func EncodeCard(src image.Image) ([]byte, error) {
	return encode(src, CardQuality)
}

// EncodeLossless encodes a picture as lossless WebP.
func EncodeLossless(src image.Image) ([]byte, error) {
	return encode(src, 0)
}

// EncodeThumb scales a picture to thumbnail width as lossy WebP.
func EncodeThumb(src image.Image) ([]byte, error) {
	return EncodeScaled(src, ThumbWidth, ThumbQuality)
}

// EncodeScaled scales a picture to a width as lossy WebP.
func EncodeScaled(src image.Image, width, quality int) ([]byte, error) {
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, errors.New("images: empty picture")
	}
	if b.Dx() <= width {
		return encode(src, quality)
	}
	height := (b.Dy()*width + b.Dx()/2) / b.Dx()
	scaled := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, draw.Over, nil)
	return encode(scaled, quality)
}

func encode(src image.Image, quality int) ([]byte, error) {
	var out bytes.Buffer
	if err := libwebp.Encode(&out, src, webpoptions.EncodingOptions{Quality: quality, EncodingPreset: webpoptions.EncodingPresetPicture}); err != nil {
		return nil, fmt.Errorf("images: webp encode: %w", err)
	}
	return out.Bytes(), nil
}
