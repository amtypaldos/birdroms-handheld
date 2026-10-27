// Package downloadui draws the download progress screen. The queue itself
// (folder mapping, atomic writes, resume) lives in internal/download and is
// tested there; this package runs it off the SDL thread, shows each title with
// its status and byte count, and turns B into a context cancel.
package downloadui

import (
	"context"
	"fmt"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/download"
	"github.com/amtypaldos/birdroms-handheld/ui/theme"
)

// Fonts are the sizes this screen uses (same Chakra Petch file).
type Fonts struct {
	Small, Tiny *ttf.Font
}

// Outcome is how the screen ended.
type Outcome int

const (
	// Back: return to browse.
	Back Outcome = iota
	// Quit: the window was closed.
	Quit
)

const (
	screenW, screenH = 640, 480
	margin           = 28
	listTop          = 56
	rowH             = 44
	maxRows          = 8
)

// Show runs q on its own goroutine and draws it until the user leaves. B (or
// the menu button) cancels the queue: finished files and the in-flight .part
// stay, later items stay pending. While the cancel is taking effect the screen
// keeps drawing; it returns once the queue has stopped. After the queue has
// finished on its own, B or A returns.
func Show(r *sdl.Renderer, f Fonts, q *download.Queue) Outcome {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		q.Run(ctx)
	}()

	s := &screen{r: r, f: f}
	cancelled := false
	for {
		finished := q.Finished()
		for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
			switch e := ev.(type) {
			case *sdl.QuitEvent:
				cancel() // do not wait: the process is exiting
				return Quit
			case *sdl.ControllerButtonEvent:
				if e.State == sdl.PRESSED && leaves(e.Button, finished) {
					cancelled = cancelled || !finished
					cancel()
				}
			case *sdl.KeyboardEvent:
				if e.Type == sdl.KEYDOWN && e.Repeat == 0 {
					name := sdl.GetKeyName(e.Keysym.Sym)
					if name == "X" || name == "Escape" || (name == "Z" && finished) { // B, Menu, A on the dev keyboard
						cancelled = cancelled || !finished
						cancel()
					}
				}
			}
		}
		if finished || cancelled {
			select {
			case <-stopped:
				return Back
			default:
			}
		}
		s.draw(q.Snapshot(), finished, cancelled)
		s.frame++
		sdl.Delay(16)
	}
}

// leaves reports whether a controller button leaves the screen: B and the
// menu button always (cancelling a running queue); A only once the queue has
// finished, so a stray A cannot stop a download.
func leaves(b uint8, finished bool) bool {
	switch sdl.GameControllerButton(b) {
	case sdl.CONTROLLER_BUTTON_B, sdl.CONTROLLER_BUTTON_GUIDE:
		return true
	case sdl.CONTROLLER_BUTTON_A:
		return finished
	}
	return false
}

type screen struct {
	r     *sdl.Renderer
	f     Fonts
	top   int // first visible row
	frame int
}

func (s *screen) draw(items []download.Item, finished, cancelled bool) {
	s.r.SetDrawColor(theme.Bg.R, theme.Bg.G, theme.Bg.B, 255)
	s.r.Clear()

	done, failed, active := 0, 0, -1
	for i, it := range items {
		switch it.Status {
		case download.Done:
			done++
		case download.Failed:
			failed++
		case download.Active:
			active = i
		}
	}
	title := fmt.Sprintf("Downloading  %d/%d", done+failed, len(items))
	switch {
	case cancelled && !finished:
		title = "Stopping" + strings.Repeat(".", (s.frame/20)%4)
	case finished:
		title = fmt.Sprintf("%d of %d downloaded", done, len(items))
		if failed > 0 {
			title += fmt.Sprintf(", %d failed", failed)
		}
	}
	s.text(s.f.Small, title, theme.Neon, margin, 12)

	if active >= 0 {
		if active < s.top {
			s.top = active
		} else if active >= s.top+maxRows {
			s.top = active - maxRows + 1
		}
	}
	for i := s.top; i < len(items) && i < s.top+maxRows; i++ {
		s.drawItem(items[i], int32(listTop+(i-s.top)*rowH))
	}

	hint := "B stop"
	if finished {
		hint = "B back"
	}
	s.text(s.f.Tiny, hint, theme.Dimmed, margin, screenH-26)
}

func (s *screen) drawItem(it download.Item, y int32) {
	col := theme.Dimmed
	if it.Status == download.Active {
		col = theme.Neon
	}
	if it.Status == download.Failed {
		col = theme.Pink
	}
	s.text(s.f.Small, s.fit(s.f.Small, it.ROM.Title, screenW-2*margin-170), col, margin, y)
	s.textRight(s.f.Tiny, statusText(it), col, screenW-margin, y+6)

	// Second line: where it goes, flagged when the platform has no folder; or why it failed.
	detail, dcol := "-> Roms/"+it.Dir, theme.Dimmed
	switch {
	case it.Status == download.Failed && it.Err != "":
		detail, dcol = s.fit(s.f.Tiny, it.Err, screenW-2*margin), theme.Pink
	case !it.Known:
		detail, dcol = "unknown platform -> Roms/"+it.Dir, theme.Pink
	}
	s.text(s.f.Tiny, detail, dcol, margin, y+24)

	if it.Status == download.Active && it.Total > 0 {
		w := int32(screenW - 2*margin)
		s.fill(theme.Panel, &sdl.Rect{X: margin, Y: y + rowH - 6, W: w, H: 3})
		frac := float64(it.Written) / float64(it.Total)
		if frac > 1 {
			frac = 1
		}
		s.fill(theme.Neon, &sdl.Rect{X: margin, Y: y + rowH - 6, W: int32(float64(w) * frac), H: 3})
	}
}

// statusText is the right-hand status of a row: the byte count while
// downloading (with the total when known), else the status word.
func statusText(it download.Item) string {
	switch {
	case it.Status == download.Active || (it.Status == download.Cancelled && it.Written > 0):
		if it.Total > 0 {
			return download.FormatBytes(it.Written) + " / " + download.FormatBytes(it.Total)
		}
		return download.FormatBytes(it.Written)
	case it.Status == download.Done && it.Skipped:
		return "already there"
	case it.Status == download.Done:
		return "done  " + download.FormatBytes(it.Written)
	}
	return it.Status.String()
}

// ---- primitives (as in ui/browse) ------------------------------------------

func (s *screen) fill(c sdl.Color, rc *sdl.Rect) {
	s.r.SetDrawColor(c.R, c.G, c.B, 255)
	s.r.FillRect(rc)
}

func (s *screen) text(f *ttf.Font, str string, c sdl.Color, x, y int32) {
	if str == "" {
		return
	}
	surf, err := f.RenderUTF8Blended(str, c)
	if err != nil {
		return
	}
	defer surf.Free()
	tex, err := s.r.CreateTextureFromSurface(surf)
	if err != nil {
		return
	}
	defer tex.Destroy()
	s.r.Copy(tex, nil, &sdl.Rect{X: x, Y: y, W: surf.W, H: surf.H})
}

func (s *screen) textRight(f *ttf.Font, str string, c sdl.Color, right, y int32) {
	w, _, err := f.SizeUTF8(str)
	if err != nil {
		return
	}
	s.text(f, str, c, right-int32(w), y)
}

// fit shortens str with an ellipsis until it renders within maxW pixels.
func (s *screen) fit(f *ttf.Font, str string, maxW int) string {
	if w, _, err := f.SizeUTF8(str); err != nil || w <= maxW {
		return str
	}
	rs := []rune(str)
	for len(rs) > 1 {
		rs = rs[:len(rs)-1]
		cand := strings.TrimRight(string(rs), " ") + "…"
		if w, _, err := f.SizeUTF8(cand); err == nil && w <= maxW {
			return cand
		}
	}
	return string(rs)
}
