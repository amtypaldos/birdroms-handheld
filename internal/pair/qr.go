package pair

import (
	"errors"
	"image"

	qrcode "github.com/skip2/go-qrcode"
)

// qrPixels is the rendered QR edge length. The handheld screen is 480 high.
const qrPixels = 360

// QRImage renders payload as a black-on-white QR code (with quiet zone).
// For pairing the payload is the server-provided qr_url, not the bare code.
func QRImage(payload string) (image.Image, error) {
	if payload == "" {
		return nil, errors.New("pair: empty QR payload")
	}
	q, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return q.Image(qrPixels), nil
}
