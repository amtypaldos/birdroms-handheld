package main

import (
	"github.com/amtypaldos/birdroms-handheld/internal/firmware"
	"github.com/veandco/go-sdl2/sdl"
	"runtime"
	"testing"
)

func TestRocknixLogicalViewport(t *testing.T) {
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
	if err := configureViewport(r, firmware.Rocknix); err != nil {
		t.Fatal(err)
	}
	width, height := r.GetLogicalSize()
	if width != 640 || height != 480 {
		t.Fatalf("logical viewport = %dx%d", width, height)
	}
}
