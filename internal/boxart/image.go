package boxart

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // register decoders for image.DecodeConfig / Decode
	_ "image/jpeg"
	"image/png"
)

const (
	// MaxSide is the longest side, in pixels, art is kept at. muOS draws box
	// art in about 42% of the 640x480 screen and scales it itself, so larger
	// images only cost decode time and memory on the handheld.
	MaxSide = 600
	// maxPixels refuses an image whose decoded size would be unreasonable,
	// before decoding it.
	maxPixels = 16 << 20
)

// Prepare checks that data is an image muOS can load and returns the bytes to
// store and their file extension. PNG, JPEG, GIF and WebP up to MaxSide are
// stored untouched (muOS loads them all). A larger PNG/JPEG/GIF is scaled down
// and stored as PNG. WebP cannot be decoded with the standard library, so it
// is stored as received. Anything else is an error.
func Prepare(data []byte) ([]byte, string, error) {
	kind := sniff(data)
	if kind == "" {
		return nil, "", errors.New("boxart: cover is not a PNG, JPEG, GIF or WebP image")
	}
	if kind == "webp" {
		return data, "webp", nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("boxart: bad %s: %w", kind, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, "", fmt.Errorf("boxart: cover is %dx%d, too large", cfg.Width, cfg.Height)
	}
	if cfg.Width <= MaxSide && cfg.Height <= MaxSide {
		if kind == "jpeg" {
			kind = "jpg"
		}
		return data, kind, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("boxart: decode %s: %w", kind, err)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, scaleDown(img, MaxSide)); err != nil {
		return nil, "", err
	}
	return out.Bytes(), "png", nil
}

func sniff(d []byte) string {
	switch {
	case bytes.HasPrefix(d, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(d, []byte("\xff\xd8\xff")):
		return "jpeg"
	case bytes.HasPrefix(d, []byte("GIF8")):
		return "gif"
	case len(d) >= 12 && string(d[:4]) == "RIFF" && string(d[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

// scaleDown shrinks img so its longest side is max, averaging the source
// pixels under each destination pixel (a box filter).
func scaleDown(img image.Image, max int) *image.NRGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := max, max
	if sw >= sh {
		dh = (sh*max + sw/2) / sw
	} else {
		dw = (sw*max + sh/2) / sh
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := b.Min.Y+y*sh/dh, b.Min.Y+(y+1)*sh/dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dw; x++ {
			x0, x1 := b.Min.X+x*sw/dw, b.Min.X+(x+1)*sw/dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := img.At(sx, sy).RGBA() // premultiplied, 16 bit
					r, g, bl, a, n = r+uint64(pr), g+uint64(pg), bl+uint64(pb), a+uint64(pa), n+1
				}
			}
			dst.Set(x, y, color.RGBA64{uint16(r / n), uint16(g / n), uint16(bl / n), uint16(a / n)})
		}
	}
	return dst
}
