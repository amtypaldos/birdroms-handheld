// Command gridpreview is a development tool: it runs the real browse screen
// against a fake server with fake covers (portrait, landscape, square and
// missing) and opens a platform grid, so the tile layout can be checked on a
// desktop without a device or a server. It is not part of the app or package.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/browse"
	"github.com/amtypaldos/birdroms-handheld/internal/input"
	browseui "github.com/amtypaldos/birdroms-handheld/ui/browse"
)

var titles = []string{
	"Legend of Zelda - The Minish Cap", "Pokemon - Emerald Version", "Metroid Fusion",
	"Advance Wars", "Golden Sun", "Kirby & The Amazing Mirror", "Mario Kart - Super Circuit",
	"Castlevania - Aria of Sorrow", "Fire Emblem", "WarioWare, Inc.", "F-Zero - Maximum Velocity",
	"Final Fantasy Tactics Advance",
}

type fakeServer struct{}

func (fakeServer) ListPlatforms() ([]string, error)       { return []string{"Game Boy Advance"}, nil }
func (fakeServer) ListPlaylists() ([]api.Playlist, error) { return nil, nil }
func (fakeServer) GetPlaylist(int) (api.PlaylistDetail, error) {
	return api.PlaylistDetail{}, nil
}

func (fakeServer) ListROMs(platform string, page int) (api.ROMPage, error) {
	var out []api.ROM
	for i, t := range titles {
		r := api.ROM{ID: i + 1, Title: t, Platform: platform, Extension: ".gba"}
		switch i % 4 { // a mix of tag lines, including none at all
		case 0:
			r.Tags = []string{"USA"}
		case 1:
			r.Tags = []string{"USA", "Europe", "Rev 1", "Beta"}
		case 2:
			r.Tags = []string{"Japan", "Proto"}
		}
		r.Filename = t + " (" + strings.Join(r.Tags, ", ") + ").gba"
		if i%5 != 4 { // every fifth ROM has no cover: placeholder
			u := "http://example/cover/" + strconv.Itoa(i%5)
			r.CoverURL = &u
		}
		out = append(out, r)
	}
	return api.ROMPage{Count: len(out), Results: out}, nil
}

// GetCover returns a gradient whose shape depends on the URL: portrait box
// art, landscape, square, and a tall strip.
func (fakeServer) GetCover(url string) ([]byte, error) {
	sizes := [][2]int{{300, 420}, {420, 300}, {300, 300}, {160, 480}}
	n, _ := strconv.Atoi(url[strings.LastIndex(url, "/")+1:])
	w, h := sizes[n%len(sizes)][0], sizes[n%len(sizes)][1]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x*120/w), G: uint8(20 + y*100/h), B: uint8(160 - x*60/w), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return b.Bytes(), nil
}

func main() {
	fontPath := flag.String("font", "assets/fonts/ChakraPetch-Bold.ttf", "font file")
	shotDir := flag.String("shots", "", "write PNGs of the frames listed in -at into this directory, then exit")
	at := flag.String("at", "60,120", "comma-separated frame numbers to capture")
	flag.Parse()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}
	defer sdl.Quit()
	ttf.Init()
	defer ttf.Quit()
	win, err := sdl.CreateWindow("gridpreview", 100, 100, 640, 480, sdl.WINDOW_SHOWN)
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()
	r, err := sdl.CreateRenderer(win, -1, sdl.RENDERER_SOFTWARE)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Destroy()
	open := func(n int) *ttf.Font {
		f, err := ttf.OpenFont(*fontPath, n)
		if err != nil {
			log.Fatal(err)
		}
		return f
	}
	small, tiny := open(22), open(15)

	srv := fakeServer{}
	m := browse.New(srv)
	want := map[int]bool{}
	last := 0
	if *shotDir != "" {
		for _, f := range strings.Split(*at, ",") {
			n, _ := strconv.Atoi(strings.TrimSpace(f))
			want[n] = true
			if n > last {
				last = n
			}
		}
	}
	// Script: open the platform, select two ROMs, then (by the second shot)
	// scroll down so the grid's second page of rows shows.
	press := func(b input.Button) { go func() { _ = m.Run(m.Press(b)) }() }
	script := map[int][]input.Button{
		10:  {input.A},
		30:  {input.Right, input.A, input.Down, input.Right, input.A},
		90:  {input.Down, input.Down},
		130: {input.Y}, // select all: every tile gets the check, footer says "Y none"
		170: {input.Y}, // select none again
	}
	browseui.FrameHook = func(r *sdl.Renderer, frame int) {
		for _, b := range script[frame] {
			press(b)
		}
		if want[frame] {
			shot(r, filepath.Join(*shotDir, "grid-"+strconv.Itoa(frame)+".png"))
		}
		if *shotDir != "" && frame >= last {
			sdl.PushEvent(&sdl.QuitEvent{Type: sdl.QUIT})
		}
	}
	browseui.Show(r, browseui.Fonts{Small: small, Tiny: tiny}, m, srv, func([]api.ROM) bool { return false })
}

// shot saves the renderer's current frame as a PNG.
func shot(r *sdl.Renderer, path string) {
	w, h, _ := r.GetOutputSize()
	surf, err := sdl.CreateRGBSurfaceWithFormat(0, w, h, 32, uint32(sdl.PIXELFORMAT_ABGR8888))
	if err != nil {
		log.Printf("shot: %v", err)
		return
	}
	defer surf.Free()
	if err := r.ReadPixels(nil, uint32(sdl.PIXELFORMAT_ABGR8888), surf.Data(), int(surf.Pitch)); err != nil {
		log.Printf("shot: %v", err)
		return
	}
	img := &image.RGBA{Pix: append([]byte(nil), surf.Pixels()...), Stride: int(surf.Pitch), Rect: image.Rect(0, 0, int(w), int(h))}
	f, err := os.Create(path)
	if err != nil {
		log.Printf("shot: %v", err)
		return
	}
	defer f.Close()
	png.Encode(f, img)
}
