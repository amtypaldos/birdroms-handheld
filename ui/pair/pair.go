// Package pairui draws the first-run pairing screen (QR + short code) on the
// handheld's SDL renderer. All behavior (HTTP, polling, expiry, persistence)
// lives in internal/pair and is tested there; this package only draws and
// translates cancel buttons.
package pairui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"time"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/input"
	"github.com/amtypaldos/birdroms-handheld/internal/pair"
)

// Colors match the hello-world screen and the web UI.
var (
	bg     = sdl.Color{R: 0x0b, G: 0x0d, B: 0x17, A: 255}
	neon   = sdl.Color{R: 0x00, G: 0xf0, B: 0xff, A: 255}
	pink   = sdl.Color{R: 0xff, G: 0x2d, B: 0x95, A: 255}
	dimmed = sdl.Color{R: 0x8a, G: 0x90, B: 0xa8, A: 255}
)

// Fonts are the three sizes the screens use.
type Fonts struct {
	Big, Mid, Small *ttf.Font
}

// Outcome is how a screen ended.
type Outcome int

const (
	// Paired: the token was obtained and saved.
	Paired Outcome = iota
	// Cancelled: the user backed out (B / Start / Menu), or pairing failed
	// and the user dismissed the error.
	Cancelled
	// Quit: the window was closed.
	Quit
)

// Show runs the pairing screen until pairing finishes or the user backs out.
// The caller starts pair.Run(ctx, ..., tr) on a goroutine and passes the same
// tracker plus the cancel func for ctx. Show never blocks the SDL thread: it
// pumps events and redraws every frame.
func Show(r *sdl.Renderer, f Fonts, tr *pair.Tracker, cancel context.CancelFunc) Outcome {
	var (
		qrTex  *sdl.Texture
		qrFor  string
		frame  int
		failed bool
	)
	defer func() {
		if qrTex != nil {
			qrTex.Destroy()
		}
	}()

	for {
		if c, q := pollCancel(); q {
			cancel()
			return Quit
		} else if c {
			cancel()
			return Cancelled
		}

		s := tr.Snapshot()
		if s.Done && s.Err == nil {
			return Paired
		}
		failed = s.Done && s.Err != nil

		// (Re)build the QR texture whenever the session's URL changes.
		if s.HaveSession && s.Session.QRURL != qrFor {
			if qrTex != nil {
				qrTex.Destroy()
				qrTex = nil
			}
			qrFor = s.Session.QRURL
			if img, err := pair.QRImage(s.Session.QRURL); err == nil {
				qrTex = textureFromImage(r, img)
			}
		}

		r.SetDrawColor(bg.R, bg.G, bg.B, 255)
		r.Clear()
		switch {
		case failed:
			drawError(r, f, s.Err)
		case !s.HaveSession:
			text(r, f.Small, "Contacting server"+dots(frame), neon, 40, 200)
			text(r, f.Small, "B / START to cancel", dimmed, 40, 420)
		default:
			drawSession(r, f, s.Session, qrTex, frame)
		}
		r.Present()
		frame++
		sdl.Delay(16)
	}
}

// Message shows lines of text until the user presses B/Start/Menu (or quits).
// It reports Cancelled for a button and Quit for a window close.
func Message(r *sdl.Renderer, f Fonts, lines ...string) Outcome {
	for {
		if c, q := pollCancel(); q {
			return Quit
		} else if c {
			return Cancelled
		}
		r.SetDrawColor(bg.R, bg.G, bg.B, 255)
		r.Clear()
		y := int32(120)
		for _, l := range lines {
			text(r, f.Small, l, neon, 40, y)
			y += 36
		}
		text(r, f.Small, "B / START to exit", dimmed, 40, 420)
		r.Present()
		sdl.Delay(16)
	}
}

func drawSession(r *sdl.Renderer, f Fonts, s pair.Session, qr *sdl.Texture, frame int) {
	const qrX, qrY, qrSize = 20, 60, 360
	if qr != nil {
		r.Copy(qr, nil, &sdl.Rect{X: qrX, Y: qrY, W: qrSize, H: qrSize})
	} else {
		text(r, f.Small, "(QR unavailable)", pink, qrX, qrY+160)
	}
	const x = 396
	text(r, f.Small, "PAIR DEVICE", neon, x, 60)
	text(r, f.Small, "Scan the QR with", dimmed, x, 110)
	text(r, f.Small, "your phone, or", dimmed, x, 140)
	text(r, f.Small, "enter this code:", dimmed, x, 170)
	text(r, f.Mid, s.Code, pink, x, 210)
	left := time.Until(s.ExpiresAt).Round(time.Second)
	if left < 0 {
		left = 0
	}
	text(r, f.Small, "Waiting"+dots(frame), neon, x, 290)
	text(r, f.Small, fmt.Sprintf("expires %d:%02d", int(left.Minutes()), int(left.Seconds())%60), dimmed, x, 320)
	text(r, f.Small, "B / START to cancel", dimmed, 40, 440)
}

func drawError(r *sdl.Renderer, f Fonts, err error) {
	msg := "Pairing failed"
	if errors.Is(err, pair.ErrExpired) {
		msg = "Code expired"
	}
	text(r, f.Mid, msg, pink, 40, 150)
	text(r, f.Small, "Start the app again to retry.", dimmed, 40, 230)
	text(r, f.Small, "B / START to exit", dimmed, 40, 420)
}

func dots(frame int) string {
	n := (frame / 20) % 4
	return "...."[:n]
}

// pollCancel drains pending events and reports whether a cancel button was
// pressed or the window was closed.
func pollCancel() (cancelled, quit bool) {
	for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
		switch e := ev.(type) {
		case *sdl.QuitEvent:
			return false, true
		case *sdl.ControllerButtonEvent:
			if e.State == sdl.PRESSED &&
				(e.Button == sdl.CONTROLLER_BUTTON_START || e.Button == sdl.CONTROLLER_BUTTON_B) {
				cancelled = true
			}
		case *sdl.KeyboardEvent:
			if e.Type == sdl.KEYDOWN {
				if b, ok := input.FromKeyName(sdl.GetKeyName(e.Keysym.Sym)); ok &&
					(b == input.B || b == input.Start || b == input.Menu) {
					cancelled = true
				}
			}
		}
	}
	return cancelled, false
}

// textureFromImage uploads img as an RGBA texture; nil on failure.
func textureFromImage(r *sdl.Renderer, img image.Image) *sdl.Texture {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	// image.RGBA bytes are R,G,B,A in memory = ABGR8888 as a little-endian word.
	surf, err := sdl.CreateRGBSurfaceWithFormatFrom(unsafe.Pointer(&rgba.Pix[0]),
		int32(b.Dx()), int32(b.Dy()), 32, int32(rgba.Stride), sdl.PIXELFORMAT_ABGR8888)
	if err != nil {
		return nil
	}
	defer surf.Free()
	tex, err := r.CreateTextureFromSurface(surf) // copies pixels
	runtime.KeepAlive(rgba)
	if err != nil {
		return nil
	}
	return tex
}

func text(r *sdl.Renderer, f *ttf.Font, s string, c sdl.Color, x, y int32) {
	if s == "" {
		return
	}
	surf, err := f.RenderUTF8Blended(s, c)
	if err != nil {
		return
	}
	defer surf.Free()
	tex, err := r.CreateTextureFromSurface(surf)
	if err != nil {
		return
	}
	defer tex.Destroy()
	r.Copy(tex, nil, &sdl.Rect{X: x, Y: y, W: surf.W, H: surf.H})
}
