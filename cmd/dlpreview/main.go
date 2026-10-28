// Command dlpreview is a development tool: it runs the real download screen
// against a fake slow server and fake covers so the layout can be checked on a
// desktop without a device or a server. It is not part of the app or package.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/download"
	downloadui "github.com/amtypaldos/birdroms-handheld/ui/download"
)

type fakeServer struct{ bytesPerSec int }

func (f fakeServer) DownloadROMFrom(ctx context.Context, id int, offset int64, w io.Writer, progress func(int64)) error {
	if id == 4 {
		return errors.New("HTTP 404: this ROM's file is missing from storage")
	}
	size := int64(sizes[id])
	chunk := int64(f.bytesPerSec / 20)
	buf := make([]byte, chunk)
	written := offset
	for written < size {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		n := chunk
		if size-written < n {
			n = size - written
		}
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		written += n
		if progress != nil {
			progress(written)
		}
	}
	return nil
}

func (fakeServer) GetCover(string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 420))
	for y := 0; y < 420; y++ {
		for x := 0; x < 300; x++ {
			img.Set(x, y, color.RGBA{R: uint8(40 + x/3), G: uint8(20 + y/4), B: uint8(160 - x/4), A: 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes(), nil
}

var sizes = map[int]int{1: 16 << 20, 2: 12 << 20, 3: 8 << 20, 4: 4 << 20, 5: 6 << 20, 6: 2 << 20, 7: 10 << 20}

func main() {
	speed := flag.Int("speed", 3<<20, "fake download speed in bytes/s")
	fontPath := flag.String("font", "assets/fonts/ChakraPetch-Bold.ttf", "font file")
	shotDir := flag.String("shots", "", "write PNGs of the frames listed in -at into this directory, then exit")
	at := flag.String("at", "150,600", "comma-separated frame numbers to capture")
	flag.Parse()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}
	defer sdl.Quit()
	ttf.Init()
	defer ttf.Quit()
	win, err := sdl.CreateWindow("dlpreview", 100, 100, 640, 480, sdl.WINDOW_SHOWN)
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
	mid, small, tiny := open(34), open(22), open(15)

	mount, _ := os.MkdirTemp("", "dlpreview")
	defer os.RemoveAll(mount)
	gbaDir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(gbaDir, 0o755)
	// ROM 2 is already on the card under a tagged name.
	os.WriteFile(filepath.Join(gbaDir, "Pokemon - Emerald Version (USA, Europe).gba"), make([]byte, sizes[2]), 0o644)

	sz := func(id int) *int64 { v := int64(sizes[id]); return &v }
	cover := "http://example/cover"
	rom := func(id int, title, platform, ext string) api.ROM {
		var tags []string
		switch id % 3 { // some with tags, some without
		case 1:
			tags = []string{"USA", "Europe"}
		case 2:
			tags = []string{"Japan", "Rev 1", "Beta"}
		}
		fn := title + ext
		if len(tags) > 0 {
			fn = title + " (" + strings.Join(tags, ", ") + ")" + ext
		}
		return api.ROM{ID: id, Title: title, Filename: fn, Tags: tags, Platform: platform, Extension: ext, SizeBytes: sz(id), CoverURL: &cover}
	}
	roms := []api.ROM{
		rom(1, "Legend of Zelda - The Minish Cap", "Game Boy Advance", ".gba"),
		rom(2, "Pokemon - Emerald Version", "Game Boy Advance", ".gba"),
		rom(3, "Bomberman 64", "Nintendo 64", ".z64"),
		rom(4, "Broken Dump That Is Missing", "Nintendo 64", ".z64"),
		rom(5, "Metroid Prime", "Nintendo GameCube", ".iso"),
		rom(6, "Kirby Super Star Ultra Deluxe Remastered Edition", "Nintendo DS", ".nds"),
		rom(7, "Sonic the Hedgehog 3 & Knuckles - Complete Collection", "Sega Genesis", ".md"),
	}
	if *shotDir != "" {
		want := map[int]bool{}
		last := 0
		for _, f := range strings.Split(*at, ",") {
			n, _ := strconv.Atoi(strings.TrimSpace(f))
			want[n] = true
			if n > last {
				last = n
			}
		}
		downloadui.FrameHook = func(r *sdl.Renderer, frame int) {
			if want[frame] {
				shot(r, filepath.Join(*shotDir, "frame-"+strconv.Itoa(frame)+".png"))
			}
			if frame >= last {
				sdl.PushEvent(&sdl.QuitEvent{Type: sdl.QUIT})
			}
		}
	}
	srv := fakeServer{bytesPerSec: *speed}
	q := download.New(srv, mount, roms)
	downloadui.Show(r, downloadui.Fonts{Mid: mid, Small: small, Tiny: tiny}, q, srv)
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
