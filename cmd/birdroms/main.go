// Command birdroms is the on-device birdROMs client (muOS, SDL2).
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/version"
)

const (
	screenW, screenH = 640, 480
	fontPath         = "assets/fonts/ChakraPetch-Bold.ttf"
)

var (
	bg     = sdl.Color{R: 0x0b, G: 0x0d, B: 0x17, A: 255}
	neon   = sdl.Color{R: 0x00, G: 0xf0, B: 0xff, A: 255}
	pink   = sdl.Color{R: 0xff, G: 0x2d, B: 0x95, A: 255}
	dimmed = sdl.Color{R: 0x8a, G: 0x90, B: 0xa8, A: 255}
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	log.Printf("%s starting", version.String())
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_GAMECONTROLLER | sdl.INIT_JOYSTICK); err != nil {
		return fmt.Errorf("sdl init: %w", err)
	}
	defer sdl.Quit()
	drv, _ := sdl.GetCurrentVideoDriver()
	log.Printf("sdl video driver: %s", drv)

	if err := ttf.Init(); err != nil {
		return fmt.Errorf("ttf init: %w", err)
	}
	defer ttf.Quit()

	win, err := sdl.CreateWindow("birdROMs", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		screenW, screenH, sdl.WINDOW_SHOWN|sdl.WINDOW_FULLSCREEN_DESKTOP)
	if err != nil {
		return fmt.Errorf("window: %w", err)
	}
	defer win.Destroy()

	r, err := sdl.CreateRenderer(win, -1, sdl.RENDERER_SOFTWARE)
	if err != nil {
		return fmt.Errorf("renderer: %w", err)
	}
	defer r.Destroy()
	w, h, _ := r.GetOutputSize()
	log.Printf("output size: %dx%d", w, h)

	big, err := ttf.OpenFont(fontPath, 56)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer big.Close()
	small, err := ttf.OpenFont(fontPath, 22)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer small.Close()

	for i := 0; i < sdl.NumJoysticks(); i++ {
		if sdl.IsGameController(i) {
			if c := sdl.GameControllerOpen(i); c != nil {
				log.Printf("controller %d: %s", i, c.Name())
			}
		}
	}

	last := "press a button"
	for {
		for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
			switch e := ev.(type) {
			case *sdl.QuitEvent:
				return nil
			case *sdl.ControllerButtonEvent:
				if e.State == sdl.PRESSED {
					name := sdl.GameControllerGetStringForButton(sdl.GameControllerButton(e.Button))
					last = "button: " + name
					log.Printf("controller button %d (%s)", e.Button, name)
					if e.Button == sdl.CONTROLLER_BUTTON_START || e.Button == sdl.CONTROLLER_BUTTON_B {
						return nil
					}
				}
			case *sdl.JoyButtonEvent:
				if e.State == sdl.PRESSED {
					log.Printf("joy button %d", e.Button)
				}
			case *sdl.JoyHatEvent:
				log.Printf("joy hat %d value %d", e.Hat, e.Value)
				last = fmt.Sprintf("hat value %d", e.Value)
			case *sdl.KeyboardEvent:
				if e.Type == sdl.KEYDOWN {
					log.Printf("key %s", sdl.GetKeyName(e.Keysym.Sym))
				}
			}
		}
		r.SetDrawColor(bg.R, bg.G, bg.B, 255)
		r.Clear()
		drawText(r, big, "birdROMs", neon, 40, 150)
		drawText(r, small, version.String(), dimmed, 44, 230)
		drawText(r, small, last, pink, 44, 300)
		drawText(r, small, "B / START to exit", dimmed, 44, 420)
		r.Present()
		sdl.Delay(16)
	}
}

func drawText(r *sdl.Renderer, f *ttf.Font, s string, c sdl.Color, x, y int32) {
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
