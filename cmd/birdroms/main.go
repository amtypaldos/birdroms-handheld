// Command birdroms is the on-device birdROMs client (muOS, SDL2).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/browse"
	"github.com/amtypaldos/birdroms-handheld/internal/config"
	"github.com/amtypaldos/birdroms-handheld/internal/download"
	"github.com/amtypaldos/birdroms-handheld/internal/pair"
	"github.com/amtypaldos/birdroms-handheld/internal/version"
	browseui "github.com/amtypaldos/birdroms-handheld/ui/browse"
	downloadui "github.com/amtypaldos/birdroms-handheld/ui/download"
	pairui "github.com/amtypaldos/birdroms-handheld/ui/pair"
)

const (
	screenW, screenH = 640, 480
	fontPath         = "assets/fonts/ChakraPetch-Bold.ttf"
	// configPath is relative to the working directory: mux_launch.sh cds to the app dir.
	configPath = "config.json"
	// seedPath is written by the website into the downloaded app: which server to pair with.
	seedPath = "server.json"
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

	// On the handheld (linux) take the whole screen; on a dev machine use a 640x480 window.
	flags := uint32(sdl.WINDOW_SHOWN)
	if runtime.GOOS == "linux" {
		flags |= sdl.WINDOW_FULLSCREEN_DESKTOP
	}
	win, err := sdl.CreateWindow("birdROMs", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		screenW, screenH, flags)
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
	mid, err := ttf.OpenFont(fontPath, 34)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer mid.Close()
	tiny, err := ttf.OpenFont(fontPath, 15)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	defer tiny.Close()

	for i := 0; i < sdl.NumJoysticks(); i++ {
		if sdl.IsGameController(i) {
			if c := sdl.GameControllerOpen(i); c != nil {
				log.Printf("controller %d: %s", i, c.Name())
			}
		}
	}

	fonts := pairui.Fonts{Big: big, Mid: mid, Small: small}
	for {
		// Pair first if needed; once paired, go straight to browse.
		switch firstRun(r, fonts) {
		case pairui.Paired:
		case pairui.Cancelled, pairui.Quit:
			return nil
		}
		cfg, err := config.Load(configPath)
		if err != nil || !cfg.Paired() {
			return fmt.Errorf("config after pairing: paired=%v err=%v", cfg.Paired(), err)
		}
		client := api.New(cfg.ServerURL, cfg.DeviceToken, &http.Client{Timeout: 10 * time.Second})
		// Downloads use their own client (same server and token) with no
		// overall timeout; browse keeps the 10s one above. muOS's launch
		// script exports the ROM card mount; nothing is hardcoded here.
		romMount := os.Getenv("BIRDROMS_ROM_MOUNT")
		if romMount == "" {
			log.Printf("BIRDROMS_ROM_MOUNT is not set; downloads will fail")
		}
		dlClient := api.New(cfg.ServerURL, cfg.DeviceToken, download.HTTPClient())
		dl := func(roms []api.ROM) bool {
			q := download.New(dlClient, romMount, roms)
			// Every storage the launcher found, for the already-there check.
			q.SetLibraries(filepath.SplitList(os.Getenv("BIRDROMS_ROM_MOUNTS"))...)
			return downloadui.Show(r, downloadui.Fonts{Mid: mid, Small: small, Tiny: tiny}, q, client) == downloadui.Quit
		}
		switch browseui.Show(r, browseui.Fonts{Small: small, Tiny: tiny}, browse.New(client), client, dl) {
		case browseui.Revoked:
			// The server no longer knows this device: forget the token and
			// go back through pairing. The server URL is kept.
			log.Printf("device token rejected; re-pairing")
			cfg.DeviceToken = ""
			if err := config.Save(configPath, cfg); err != nil {
				return fmt.Errorf("clear revoked token: %w", err)
			}
		default:
			return nil
		}
	}
}

// firstRun makes sure the device is paired before the app proper starts. It
// returns Paired when a token is already stored or pairing just succeeded.
func firstRun(r *sdl.Renderer, f pairui.Fonts) pairui.Outcome {
	cfg, err := config.Load(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("config: %v", err)
		return pairui.Message(r, f, "config.json is unreadable.", "Fix or delete it, then retry.")
	}
	if cfg.Paired() {
		return pairui.Paired
	}
	server := pair.ResolveServer(os.Getenv("BIRDROMS_SERVER"), config.WithSeedServer(cfg, seedPath))
	if server == "" {
		return pairui.Message(r, f, "No server configured.", "Download birdROMs again from your", "friend's birdROMs website (Handheld", "page) and reinstall it.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := pair.NewTracker()
	go pair.Run(ctx, pair.Options{
		BaseURL:    server,
		ConfigPath: configPath,
		Client:     &http.Client{Timeout: 10 * time.Second},
	}, tr)
	return pairui.Show(r, f, tr, cancel)
}
