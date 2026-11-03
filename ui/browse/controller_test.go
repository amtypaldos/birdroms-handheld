package browseui

import (
	"github.com/amtypaldos/birdroms-handheld/internal/input"
	"github.com/veandco/go-sdl2/sdl"
	"testing"
)

func TestControllerShoulderPaging(t *testing.T) {
	for button, want := range map[sdl.GameControllerButton]input.Button{sdl.CONTROLLER_BUTTON_LEFTSHOULDER: input.L1, sdl.CONTROLLER_BUTTON_RIGHTSHOULDER: input.R1} {
		if got, ok := buttonFromController(uint8(button)); !ok || got != want {
			t.Fatalf("shoulder %v: %v %v", button, got, ok)
		}
	}
}
