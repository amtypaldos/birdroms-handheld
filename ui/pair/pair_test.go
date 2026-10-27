package pairui

import (
	"testing"

	"github.com/veandco/go-sdl2/sdl"
)

func TestIsCancelButton(t *testing.T) {
	cancel := []sdl.GameControllerButton{
		sdl.CONTROLLER_BUTTON_B,
		sdl.CONTROLLER_BUTTON_START,
		sdl.CONTROLLER_BUTTON_GUIDE, // the muOS menu button
	}
	for _, b := range cancel {
		if !isCancelButton(uint8(b)) {
			t.Errorf("button %d should cancel", b)
		}
	}
	for _, b := range []sdl.GameControllerButton{sdl.CONTROLLER_BUTTON_A, sdl.CONTROLLER_BUTTON_X, sdl.CONTROLLER_BUTTON_BACK} {
		if isCancelButton(uint8(b)) {
			t.Errorf("button %d should not cancel", b)
		}
	}
}
