package browse

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // register decoders for cover formats
	_ "image/jpeg"
	_ "image/png"
)

// DecodeCover decodes JPEG/PNG/GIF bytes and shrinks the result to fit
// maxW x maxH. It runs off the SDL thread.
func DecodeCover(data []byte, maxW, maxH int) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("browse: decode cover: %w", err)
	}
	return ScaleToFit(img, maxW, maxH), nil
}
