// Package browseui draws the browse screens (home list and cover grid) on the
// handheld's SDL renderer. All behavior (navigation, paging, selection, the
// cover cache policy) lives in internal/browse and is tested there; this
// package translates input, draws, and keeps the network off the SDL thread.
package browseui

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"strings"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/browse"
	"github.com/amtypaldos/birdroms-handheld/internal/input"
	"github.com/amtypaldos/birdroms-handheld/ui/theme"
)

// Fonts are the sizes the browse screens use (same Chakra Petch file).
type Fonts struct {
	Small, Tiny *ttf.Font
}

// Downloader runs the download queue for the selected ROMs, blocking (and
// drawing its own screen) until the user leaves it. It reports whether the
// window was closed meanwhile.
type Downloader func(roms []api.ROM) (quit bool)

// FrameHook, when set by a development tool (cmd/gridpreview), is called after
// each frame is drawn and before it is presented. It is nil in the app.
var FrameHook func(r *sdl.Renderer, frame int)

// Outcome is how the browse screen ended.
type Outcome int

const (
	// Exit: the user left with B on home or the menu button.
	Exit Outcome = iota
	// Quit: the window was closed.
	Quit
	// Revoked: the server rejected the device token.
	Revoked
)

const (
	screenW, screenH = 640, 480

	// Home list.
	homeTop    = 52
	homeRowH   = 34
	homeRows   = 11
	homeMargin = 28

	// Grid.
	cellW, cellH = 148, 190 // square box + title + a second text line
	gridX, gridY = 24, 52
	boxW, boxH   = 136, 136 // square cover box inside a cell
)

// coverTex is a cached cover. A nil *coverTex in the cache means the fetch
// failed; the cell keeps its placeholder until the entry is evicted.
type coverTex struct {
	tex  *sdl.Texture
	w, h int32
}

type screen struct {
	r      *sdl.Renderer
	f      Fonts
	m      *browse.Model
	dl     Downloader
	covers *coverLoader
	cache  *browse.Cache[*coverTex]

	inflight map[int]bool // cover fetches queued or running; SDL thread only
	homeTop  int          // first visible home row
	gridTop  int          // first visible grid row
	frame    int
	wantDL   bool // Start was pressed with a selection; SDL thread only
}

// Show runs the browse screen until the user leaves it. m must be fresh from
// browse.New; cs fetches covers (normally the same *api.Client the model
// reads from); dl runs the download queue when Start is pressed with a
// non-empty selection. It never blocks the SDL thread on the network: page and
// cover fetches run on their own goroutines, and dl owns its own goroutine for
// the downloads.
func Show(r *sdl.Renderer, f Fonts, m *browse.Model, cs CoverSource, dl Downloader) Outcome {
	s := &screen{
		r: r, f: f, m: m, dl: dl,
		covers:   newCoverLoader(cs),
		inflight: map[int]bool{},
	}
	s.cache = browse.NewCache(browse.CoverCacheSize, func(_ int, c *coverTex) {
		if c != nil && c.tex != nil {
			c.tex.Destroy()
		}
	})
	defer s.cache.Clear()
	defer s.covers.Close()

	s.start(m.Start())
	for {
		if s.pump() {
			return Quit
		}
		if s.wantDL {
			s.wantDL = false
			if sel := m.Selected(); len(sel) > 0 && s.dl(sel) {
				return Quit
			}
		}
		v := m.View()
		if v.Unauthorized || s.covers.revoked.Load() {
			return Revoked
		}
		if m.Done() {
			return Exit
		}
		s.draw(v)
		s.frame++
		sdl.Delay(16)
	}
}

// start runs an effect off the SDL thread. Errors are recorded in the model.
func (s *screen) start(e browse.Effect) {
	if e.Kind == browse.EffectNone {
		return
	}
	go func() { _ = s.m.Run(e) }()
}

// pump drains SDL events into model presses. It reports a window close.
func (s *screen) pump() (quit bool) {
	for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
		switch e := ev.(type) {
		case *sdl.QuitEvent:
			return true
		case *sdl.ControllerButtonEvent:
			if e.State == sdl.PRESSED {
				if b, ok := buttonFromController(e.Button); ok {
					s.press(b)
				}
			}
		case *sdl.KeyboardEvent:
			if e.Type != sdl.KEYDOWN {
				continue
			}
			b, ok := input.FromKeyName(sdl.GetKeyName(e.Keysym.Sym))
			if !ok {
				continue
			}
			if e.Repeat != 0 && b != input.Up && b != input.Down && b != input.Left && b != input.Right {
				continue // holding A must not flip the selection back and forth
			}
			s.press(b)
		}
	}
	return false
}

// press applies a button. Start is handled here, not in the model: with a
// selection it asks the main loop to run the download queue; without one it
// does nothing.
func (s *screen) press(b input.Button) {
	if b == input.Start {
		if len(s.m.Selected()) > 0 {
			s.wantDL = true
		}
		return
	}
	s.start(s.m.Press(b))
}

// buttonFromController maps SDL controller buttons to logical buttons. Guide
// is the muOS menu button (guide:b12 on the RG40XX-V).
func buttonFromController(b uint8) (input.Button, bool) {
	switch sdl.GameControllerButton(b) {
	case sdl.CONTROLLER_BUTTON_DPAD_UP:
		return input.Up, true
	case sdl.CONTROLLER_BUTTON_DPAD_DOWN:
		return input.Down, true
	case sdl.CONTROLLER_BUTTON_DPAD_LEFT:
		return input.Left, true
	case sdl.CONTROLLER_BUTTON_DPAD_RIGHT:
		return input.Right, true
	case sdl.CONTROLLER_BUTTON_A:
		return input.A, true
	case sdl.CONTROLLER_BUTTON_B:
		return input.B, true
	case sdl.CONTROLLER_BUTTON_START:
		return input.Start, true
	case sdl.CONTROLLER_BUTTON_GUIDE:
		return input.Menu, true
	}
	return 0, false
}

// ---- drawing ---------------------------------------------------------------

func (s *screen) draw(v browse.View) {
	s.r.SetDrawColor(theme.Bg.R, theme.Bg.G, theme.Bg.B, 255)
	s.r.Clear()
	if v.Mode == browse.ModeHome {
		s.drawHome(v)
	} else {
		s.drawGrid(v)
	}
	if FrameHook != nil {
		FrameHook(s.r, s.frame)
	}
	s.r.Present()
}

func (s *screen) drawHome(v browse.View) {
	s.text(s.f.Small, "birdROMs", theme.Neon, homeMargin, 12)

	switch {
	case v.Cursor < 0 && v.Loading:
		s.text(s.f.Small, "Loading"+dots(s.frame), theme.Neon, homeMargin, homeTop+8)
	case v.Cursor < 0 && v.Err != "":
		s.text(s.f.Small, v.Err, theme.Pink, homeMargin, homeTop+8)
		s.text(s.f.Small, "Press A to retry", theme.Dimmed, homeMargin, homeTop+8+homeRowH)
	default:
		if v.Cursor >= 0 {
			if v.Cursor < s.homeTop {
				s.homeTop = v.Cursor
			} else if v.Cursor >= s.homeTop+homeRows {
				s.homeTop = v.Cursor - homeRows + 1
			}
			// Keep the section header above the first selectable row in view.
			if s.homeTop > 0 && s.homeTop == v.Cursor && v.Rows[s.homeTop-1].Kind == browse.RowHeader {
				s.homeTop--
			}
		}
		for i := s.homeTop; i < len(v.Rows) && i < s.homeTop+homeRows; i++ {
			s.drawHomeRow(v.Rows[i], int32(homeTop+(i-s.homeTop)*homeRowH), i == v.Cursor)
		}
	}

	switch {
	case v.Loading && v.Cursor >= 0:
		s.text(s.f.Tiny, "Loading"+dots(s.frame), theme.Neon, homeMargin, screenH-26)
	case v.Err != "" && v.Cursor >= 0:
		s.text(s.f.Tiny, v.Err, theme.Pink, homeMargin, screenH-26)
	default:
		s.text(s.f.Tiny, "A open    B exit", theme.Dimmed, homeMargin, screenH-26)
	}
	if n := len(s.m.Selected()); n > 0 {
		s.textRight(s.f.Tiny, fmt.Sprintf("%d selected", n), theme.Pink, screenW-homeMargin, screenH-26)
	}
}

func (s *screen) drawHomeRow(row browse.Row, y int32, focused bool) {
	switch row.Kind {
	case browse.RowHeader:
		s.text(s.f.Tiny, strings.ToUpper(row.Text), theme.Pink, homeMargin, y+8)
	case browse.RowNote:
		s.text(s.f.Small, row.Text, theme.Dimmed, homeMargin+16, y+2)
	default:
		col := theme.Dimmed
		if focused {
			col = theme.Neon
			s.fill(theme.Panel, &sdl.Rect{X: homeMargin - 8, Y: y, W: screenW - 2*(homeMargin-8), H: homeRowH - 2})
			s.fill(theme.Neon, &sdl.Rect{X: homeMargin - 8, Y: y, W: 4, H: homeRowH - 2})
		}
		label := s.fit(s.f.Small, row.Text, screenW-2*homeMargin-90)
		s.text(s.f.Small, label, col, homeMargin+16, y+2)
		if row.Kind == browse.RowPlaylist {
			s.textRight(s.f.Small, fmt.Sprint(row.Count), theme.Dimmed, screenW-homeMargin, y+2)
		}
	}
}

func (s *screen) drawGrid(v browse.View) {
	s.text(s.f.Small, s.fit(s.f.Small, v.Title, 420), theme.Neon, 28, 12)
	if n := len(s.m.Selected()); n > 0 {
		s.textRight(s.f.Small, fmt.Sprintf("%d selected", n), theme.Pink, screenW-28, 12)
	}

	if len(v.ROMs) == 0 {
		switch {
		case v.Loading:
			s.text(s.f.Small, "Loading"+dots(s.frame), theme.Neon, 28, 200)
		case v.Empty != "":
			s.text(s.f.Small, v.Empty, theme.Dimmed, 28, 200)
		}
		s.text(s.f.Tiny, "B back", theme.Dimmed, 28, screenH-26)
		return
	}

	s.gridTop = browse.ScrollRow(v.Focus/browse.Cols, s.gridTop)
	first, end := browse.VisibleRange(s.gridTop, len(v.ROMs))
	s.updateCovers(v.ROMs, first, end)

	for i := first; i < end; i++ {
		col, row := i%browse.Cols, i/browse.Cols-s.gridTop
		x := int32(gridX + col*cellW)
		y := int32(gridY + row*cellH)
		s.drawCell(v.ROMs[i], x, y, i == v.Focus, v.Selected[v.ROMs[i].ID])
	}

	switch {
	case v.Err != "":
		s.text(s.f.Tiny, v.Err, theme.Pink, 28, screenH-26)
	case len(s.m.Selected()) > 0:
		s.text(s.f.Tiny, "A select    Start download    B back", theme.Dimmed, 28, screenH-26)
	default:
		s.text(s.f.Tiny, "A select    B back", theme.Dimmed, 28, screenH-26)
	}
	s.textRight(s.f.Tiny, fmt.Sprintf("%d/%d", v.Focus+1, len(v.ROMs)), theme.Dimmed, screenW-28, screenH-26)
}

func (s *screen) drawCell(rom api.ROM, x, y int32, focused, selected bool) {
	box := &sdl.Rect{X: x + 6, Y: y + 4, W: boxW, H: boxH}
	s.fill(theme.Panel, box)

	var ct *coverTex
	if c, ok := s.cache.Get(rom.ID); ok {
		ct = c
	}
	if ct != nil && ct.tex != nil {
		s.r.Copy(ct.tex, nil, &sdl.Rect{
			X: box.X + (box.W-ct.w)/2, Y: box.Y + (box.H-ct.h)/2, W: ct.w, H: ct.h,
		})
	} else {
		// Placeholder: missing, loading or failed cover.
		s.outline(theme.Dimmed, box, 1)
		if ext := strings.ToUpper(rom.Extension); ext != "" {
			s.textCenter(s.f.Tiny, ext, theme.Dimmed, box.X+box.W/2, box.Y+box.H/2-9)
		}
	}

	s.text(s.f.Tiny, s.fit(s.f.Tiny, rom.Title, cellW-12), pickColor(focused), x+6, y+boxH+8)

	if selected {
		s.outline(theme.Pink, box, 2)
		s.drawCheck(box.X+box.W-26, box.Y+4)
	}
	if focused {
		s.outline(theme.Neon, &sdl.Rect{X: x + 1, Y: y + 1, W: cellW - 2, H: cellH - 2}, 3)
	}
}

func pickColor(focused bool) sdl.Color {
	if focused {
		return theme.Neon
	}
	return theme.Dimmed
}

// drawCheck draws the "selected" badge: a pink square with a check mark.
func (s *screen) drawCheck(x, y int32) {
	s.fill(theme.Pink, &sdl.Rect{X: x, Y: y, W: 22, H: 22})
	s.r.SetDrawColor(theme.Bg.R, theme.Bg.G, theme.Bg.B, 255)
	for d := int32(0); d < 2; d++ {
		s.r.DrawLine(x+5, y+11+d, x+9, y+15+d)
		s.r.DrawLine(x+9, y+15+d, x+17, y+6+d)
	}
}

// updateCovers applies finished cover work and asks for covers of the ROMs
// that are on screen (and only those) that are not cached or queued.
func (s *screen) updateCovers(roms []api.ROM, first, end int) {
	for _, res := range s.covers.drain() {
		delete(s.inflight, res.id)
		if res.skipped {
			continue
		}
		var ct *coverTex
		if res.img != nil {
			if tex := textureFromImage(s.r, res.img); tex != nil {
				b := res.img.Bounds()
				ct = &coverTex{tex: tex, w: int32(b.Dx()), h: int32(b.Dy())}
			}
		}
		s.cache.Put(res.id, ct) // nil marks a failure so it is not refetched every frame
	}

	want := browse.WantCovers(roms, first, end)
	ids := make(map[int]bool, len(want))
	for _, r := range want {
		ids[r.ID] = true
	}
	s.covers.setWanted(ids)
	for _, r := range want {
		if _, ok := s.cache.Get(r.ID); ok || s.inflight[r.ID] {
			continue
		}
		if s.covers.enqueue(r.ID, *r.CoverURL) {
			s.inflight[r.ID] = true
		}
	}
}

// ---- primitives ------------------------------------------------------------

func (s *screen) fill(c sdl.Color, rc *sdl.Rect) {
	s.r.SetDrawColor(c.R, c.G, c.B, 255)
	s.r.FillRect(rc)
}

func (s *screen) outline(c sdl.Color, rc *sdl.Rect, thick int32) {
	s.r.SetDrawColor(c.R, c.G, c.B, 255)
	for i := int32(0); i < thick; i++ {
		s.r.DrawRect(&sdl.Rect{X: rc.X + i, Y: rc.Y + i, W: rc.W - 2*i, H: rc.H - 2*i})
	}
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

func (s *screen) textCenter(f *ttf.Font, str string, c sdl.Color, cx, y int32) {
	w, _, err := f.SizeUTF8(str)
	if err != nil {
		return
	}
	s.text(f, str, c, cx-int32(w)/2, y)
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

func dots(frame int) string {
	n := (frame / 20) % 4
	return "...."[:n]
}

// textureFromImage uploads img as an RGBA texture; nil on failure.
func textureFromImage(r *sdl.Renderer, img image.Image) *sdl.Texture {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil
	}
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	// image.RGBA bytes are R,G,B,A in memory = ABGR8888 as a little-endian word.
	surf, err := sdl.CreateRGBSurfaceWithFormatFrom(unsafe.Pointer(&rgba.Pix[0]),
		int32(b.Dx()), int32(b.Dy()), 32, int32(rgba.Stride), sdl.PIXELFORMAT_ABGR8888)
	if err != nil {
		return nil
	}
	defer surf.Free()
	tex, err := r.CreateTextureFromSurface(surf) // copies pixels
	runtime.KeepAlive(rgba)
	if err != nil {
		return nil
	}
	return tex
}
