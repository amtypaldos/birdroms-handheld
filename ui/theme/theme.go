// Package theme holds the color tokens shared by the handheld screens. They
// match the web UI (dark / neon).
package theme

import "github.com/veandco/go-sdl2/sdl"

var (
	Bg     = sdl.Color{R: 0x0b, G: 0x0d, B: 0x17, A: 255}
	Neon   = sdl.Color{R: 0x00, G: 0xf0, B: 0xff, A: 255}
	Pink   = sdl.Color{R: 0xff, G: 0x2d, B: 0x95, A: 255}
	Dimmed = sdl.Color{R: 0x8a, G: 0x90, B: 0xa8, A: 255}
	// Panel fills a cover placeholder; a step lighter than Bg.
	Panel = sdl.Color{R: 0x16, G: 0x1a, B: 0x2c, A: 255}
)
