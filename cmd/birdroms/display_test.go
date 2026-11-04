package main

import (
	"github.com/amtypaldos/birdroms-handheld/internal/firmware"
	"github.com/veandco/go-sdl2/sdl"
	"runtime"
	"testing"
)

func TestLinuxLogicalViewport(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("SDL_VIDEODRIVER", "dummy")
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		t.Fatal(err)
	}
	defer sdl.Quit()
	w, err := sdl.CreateWindow("test", 0, 0, 1280, 720, sdl.WINDOW_HIDDEN)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Destroy()
	r, err := sdl.CreateRenderer(w, -1, sdl.RENDERER_SOFTWARE)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Destroy()
	for _, target := range []firmware.Target{firmware.Rocknix, firmware.Batocera} {
		if err := r.SetLogicalSize(0, 0); err != nil {
			t.Fatal(err)
		}
		if err := configureViewport(r, target); err != nil {
			t.Fatal(err)
		}
		width, height := r.GetLogicalSize()
		if width != 640 || height != 480 {
			t.Fatalf("%s logical viewport = %dx%d", target, width, height)
		}

	}
}
