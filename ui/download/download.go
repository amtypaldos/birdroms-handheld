// Package downloadui draws the download progress screen. The queue itself
// (folder mapping, atomic writes, resume) lives in internal/download and is
// tested there; this package runs it off the SDL thread, shows the active ROM
// as a hero card (cover, progress, speed, ETA) above the queue, and turns B
// into a context cancel.
package downloadui

import (
	"context"
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"

	"github.com/amtypaldos/birdroms-handheld/internal/browse"
	"github.com/amtypaldos/birdroms-handheld/internal/download"
	"github.com/amtypaldos/birdroms-handheld/ui/theme"
)

// Fonts are the sizes this screen uses (same Chakra Petch file).
type Fonts struct {
	Mid, Small, Tiny *ttf.Font
}

// CoverSource fetches cover bytes; *api.Client satisfies it. It may be nil, in
// which case the hero card shows a placeholder.
type CoverSource interface {
	GetCover(coverURL string) ([]byte, error)
}

// FrameHook, when set by a development tool (cmd/dlpreview), is called after
// each frame is drawn and before it is presented. It is nil in the app.
var FrameHook func(r *sdl.Renderer, frame int)

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

	heroY, heroH = 62, 178
	coverBox     = 150
	listTop      = 250
	rowH         = 36
	maxRows      = 5
)

// Show runs q on its own goroutine and draws it until the user leaves. B (or
// the menu button) cancels the queue: finished files and the in-flight .part
// stay, later items stay pending. While the cancel is taking effect the screen
// keeps drawing; it returns once the queue has stopped. After the queue has
// finished on its own, B or A returns.
func Show(r *sdl.Renderer, f Fonts, q *download.Queue, cs CoverSource) Outcome {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		q.Run(ctx)
	}()

	s := &screen{r: r, f: f, rate: download.NewRate(), rateID: -1, hero: newHeroCover(cs)}
	defer s.hero.release()
	cancelled, userLeft := false, false
	for {
		finished := q.Finished()
		leave := func() {
			if finished {
				userLeft = true // the summary was read; go back
			} else {
				cancelled = true
				cancel()
			}
		}
		for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
			switch e := ev.(type) {
			case *sdl.QuitEvent:
				cancel() // do not wait: the process is exiting
				return Quit
			case *sdl.ControllerButtonEvent:
				if e.State == sdl.PRESSED && leaves(e.Button, finished) {
					leave()
				}
			case *sdl.KeyboardEvent:
				if e.Type == sdl.KEYDOWN && e.Repeat == 0 {
					name := sdl.GetKeyName(e.Keysym.Sym)
					if name == "X" || name == "Escape" || (name == "Z" && finished) { // B, Menu, A on the dev keyboard
						leave()
					}
				}
			}
		}
		stoppedNow := false
		select {
		case <-stopped:
			stoppedNow = true
		default:
		}
		if shouldReturn(finished, userLeft, cancelled, stoppedNow) {
			return Back
		}
		s.draw(q.Snapshot(), finished, cancelled)
		s.frame++
		sdl.Delay(16)
	}
}

// shouldReturn reports whether Show may return to browse. A queue that finished
// on its own keeps the summary on screen until the user leaves; a cancelled one
// returns as soon as it has actually stopped. Never before the queue goroutine
// is done, so no download is left running behind browse.
func shouldReturn(finished, userLeft, cancelled, stopped bool) bool {
	if !stopped {
		return false
	}
	return (finished && userLeft) || cancelled
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
	r      *sdl.Renderer
	f      Fonts
	top    int // first visible queue row
	frame  int
	rate   *download.Rate
	rateID int // ROM id the rate tracker is measuring
	hero   *heroCover
}

// Palette extras on top of the shared theme.
var (
	white = sdl.Color{R: 0xff, G: 0xff, B: 0xff, A: 255}
	track = sdl.Color{R: 0x0e, G: 0x11, B: 0x1f, A: 255}
)

func (s *screen) pulse(period float64) float64 { // 0..1
	return 0.5 + 0.5*math.Sin(float64(s.frame)*2*math.Pi/period)
}

func (s *screen) draw(items []download.Item, finished, cancelled bool) {
	s.r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	s.r.SetDrawColor(theme.Bg.R, theme.Bg.G, theme.Bg.B, 255)
	s.r.Clear()

	done, failed, skipped, active := 0, 0, 0, -1
	var moved int64
	for i, it := range items {
		switch it.Status {
		case download.Done:
			done++
			if it.Skipped {
				skipped++
			} else {
				moved += it.Written
			}
		case download.Failed:
			failed++
		case download.Active:
			active = i
		}
	}

	// Header: label, counter and the overall bar.
	label := "DOWNLOADING"
	switch {
	case cancelled && !finished:
		label = "STOPPING" + strings.Repeat(".", (s.frame/20)%4)
	case finished && failed > 0:
		label = "FINISHED WITH ERRORS"
	case finished:
		label = "ALL DONE"
	}
	s.text(s.f.Small, label, headerColor(finished, failed), margin, 12)
	s.textRight(s.f.Small, fmt.Sprintf("%d / %d", done+failed, len(items)), theme.Dimmed, screenW-margin, 12)
	s.bar(margin, 46, screenW-2*margin, 4, download.Progress(items), false)

	if finished || (cancelled && active < 0) {
		s.drawSummary(items, done-skipped, skipped, failed, moved, finished)
	} else if active >= 0 {
		s.drawHero(items[active])
	} else {
		s.drawIdleHero()
	}

	// Queue list, kept scrolled so the active row stays visible.
	if active >= 0 {
		if active < s.top {
			s.top = active
		} else if active >= s.top+maxRows {
			s.top = active - maxRows + 1
		}
	}
	for i := s.top; i < len(items) && i < s.top+maxRows; i++ {
		s.drawRow(items[i], int32(listTop+(i-s.top)*rowH), i == active)
	}
	if more := len(items) - (s.top + maxRows); more > 0 {
		s.textRight(s.f.Tiny, fmt.Sprintf("+%d more", more), theme.Dimmed, screenW-margin, listTop+maxRows*rowH-4)
	}

	// Footer: the first failure's reason (if any) and the button hint.
	for _, it := range items {
		if it.Status == download.Failed && it.Err != "" {
			s.text(s.f.Tiny, s.fit(s.f.Tiny, "! "+it.Err, screenW-2*margin), theme.Pink, margin, screenH-44)
			break
		}
	}
	defer func() {
		if FrameHook != nil {
			FrameHook(s.r, s.frame)
		}
		s.r.Present()
	}()
	s.chip("B", margin, screenH-26)
	hint := "Stop"
	if finished {
		hint = "Back"
		s.chip("A", margin+78, screenH-26)
		s.text(s.f.Tiny, "Done", theme.Dimmed, margin+78+24, screenH-26)
	}
	s.text(s.f.Tiny, hint, theme.Dimmed, margin+24, screenH-26)
}

func headerColor(finished bool, failed int) sdl.Color {
	if finished && failed > 0 {
		return theme.Pink
	}
	return theme.Neon
}

// drawHero is the card for the item being downloaded now.
func (s *screen) drawHero(it download.Item) {
	if s.rateID != it.ROM.ID {
		s.rate, s.rateID = download.NewRate(), it.ROM.ID
	}
	s.rate.Update(time.Now(), it.Written)
	coverURL := ""
	if it.ROM.CoverURL != nil {
		coverURL = *it.ROM.CoverURL
	}
	s.hero.want(it.ROM.ID, coverURL)

	s.panel(heroY, true)
	cx, cy := int32(margin+14), int32(heroY+14)
	s.drawCover(cx, cy)

	x := int32(margin + 14 + coverBox + 18)
	w := screenW - margin - 14 - int(x)
	l1, l2 := s.wrap2(s.f.Small, it.ROM.Title, w)
	s.text(s.f.Small, l1, white, x, heroY+10)
	s.text(s.f.Small, l2, white, x, heroY+38)

	folder, fcol := "Roms/"+it.Dir, theme.Dimmed
	if !it.Known {
		folder, fcol = "unknown platform -> Roms/"+it.Dir, theme.Pink
	}
	s.text(s.f.Tiny, s.fit(s.f.Tiny, folder, w), fcol, x, heroY+68)

	frac := 0.0
	if it.Total > 0 {
		frac = math.Min(1, float64(it.Written)/float64(it.Total))
		s.text(s.f.Mid, fmt.Sprintf("%d%%", int(frac*100)), theme.Neon, x, heroY+86)
	}
	var stats []string
	if bps := s.rate.BytesPerSec(); bps > 0 {
		stats = append(stats, download.FormatRate(bps))
		if eta, ok := s.rate.ETA(it.Total - it.Written); ok && it.Total > 0 {
			stats = append(stats, download.FormatDuration(eta)+" left")
		}
	}
	s.textRight(s.f.Tiny, strings.Join(stats, "  .  "), theme.Dimmed, screenW-margin-14, heroY+100)
	if it.Total > 0 {
		s.bar(int(x), heroY+132, w, 12, frac, true)
		s.text(s.f.Tiny, download.FormatBytes(it.Written)+" / "+download.FormatBytes(it.Total), theme.Dimmed, x, heroY+150)
	} else {
		s.bar(int(x), heroY+132, w, 12, s.pulse(60), true) // size unknown: breathe instead of filling
		s.text(s.f.Tiny, download.FormatBytes(it.Written), theme.Dimmed, x, heroY+150)
	}
}

// drawIdleHero is the card between items (the next one has not started yet).
func (s *screen) drawIdleHero() {
	s.panel(heroY, false)
	s.drawCover(margin+14, heroY+14)
	s.text(s.f.Small, "Getting ready"+strings.Repeat(".", (s.frame/20)%4), theme.Dimmed, margin+14+coverBox+18, heroY+70)
}

// drawSummary is the card shown when the queue has ended.
func (s *screen) drawSummary(items []download.Item, fetched, skipped, failed int, moved int64, finished bool) {
	s.panel(heroY, finished && failed == 0)
	col := theme.Neon
	if failed > 0 {
		col = theme.Pink
	}
	// A big drawn mark in the cover box: a check, or a cross when anything failed.
	bx, by := int32(margin+14), int32(heroY+14)
	s.fill(track, &sdl.Rect{X: bx, Y: by, W: coverBox, H: coverBox})
	if failed > 0 {
		s.cross(bx+coverBox/2, by+coverBox/2, 38, 6, col)
	} else {
		s.check(bx+coverBox/2, by+coverBox/2, 46, 7, col)
	}
	x := int32(margin + 14 + coverBox + 18)
	head := fmt.Sprintf("%d new", fetched)
	if !finished {
		head = "Stopped"
	}
	s.text(s.f.Mid, head, col, x, heroY+16)
	y := int32(heroY + 70)
	line := func(str string, c sdl.Color) {
		s.text(s.f.Small, str, c, x, y)
		y += 30
	}
	line(fmt.Sprintf("%d already there", skipped), theme.Dimmed)
	if failed > 0 {
		line(fmt.Sprintf("%d failed", failed), theme.Pink)
	}
	if moved > 0 {
		line(download.FormatBytes(moved)+" saved", theme.Dimmed)
	}
}

func (s *screen) drawRow(it download.Item, y int32, isActive bool) {
	col := theme.Dimmed
	switch {
	case isActive:
		col = theme.Neon
	case it.Status == download.Failed:
		col = theme.Pink
	case it.Status == download.Done && !it.Skipped:
		col = white
	}
	s.icon(it, int32(margin+8), y+10, isActive)
	s.text(s.f.Small, s.fit(s.f.Small, it.ROM.Title, screenW-2*margin-200), col, margin+34, y+2)
	st := statusText(it)
	scol := col
	if !it.Known && it.Status != download.Failed {
		st, scol = "-> "+it.Dir, theme.Pink
	}
	s.textRight(s.f.Tiny, st, scol, screenW-margin, y+8)
}

// statusText is the right-hand status of a queue row.
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
		return download.FormatBytes(it.Written)
	}
	return it.Status.String()
}

// ---- cover -----------------------------------------------------------------

// heroCover fetches and decodes the active item's cover off the SDL thread and
// uploads it as a texture on it. Only one cover is held; it is replaced when
// the active item changes.
type heroCover struct {
	src  CoverSource
	id   int
	tex  *sdl.Texture
	w, h int32
	res  chan heroResult
}

type heroResult struct {
	id  int
	img image.Image
}

func newHeroCover(src CoverSource) *heroCover {
	return &heroCover{src: src, id: -1, res: make(chan heroResult, 1)}
}

// want makes id the cover to show, starting a fetch the first time it is asked.
func (h *heroCover) want(id int, url string) {
	if id == h.id {
		return
	}
	h.id = id
	h.release()
	if h.src == nil || url == "" {
		return
	}
	go func() {
		data, err := h.src.GetCover(url)
		if err != nil {
			log.Printf("download cover %d: %v", id, err)
			return
		}
		img, err := browse.DecodeCover(data, coverBox, coverBox)
		if err != nil {
			log.Printf("download cover %d: %v", id, err)
			return
		}
		select {
		case h.res <- heroResult{id: id, img: img}:
		default:
		}
	}()
}

func (h *heroCover) release() {
	if h.tex != nil {
		h.tex.Destroy()
		h.tex = nil
	}
}

// poll uploads a finished cover if it is still the wanted one.
func (h *heroCover) poll(r *sdl.Renderer) {
	select {
	case res := <-h.res:
		if res.id != h.id {
			return
		}
		b := res.img.Bounds()
		surf, err := sdl.CreateRGBSurfaceWithFormat(0, int32(b.Dx()), int32(b.Dy()), 32, uint32(sdl.PIXELFORMAT_ABGR8888))
		if err != nil {
			return
		}
		defer surf.Free()
		pix := surf.Pixels()
		i := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				rr, gg, bb, aa := res.img.At(x, y).RGBA()
				pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(rr>>8), byte(gg>>8), byte(bb>>8), byte(aa>>8)
				i += 4
			}
		}
		if tex, err := r.CreateTextureFromSurface(surf); err == nil {
			h.release()
			h.tex, h.w, h.h = tex, int32(b.Dx()), int32(b.Dy())
		}
	default:
	}
}

// drawCover draws the hero cover (centred, aspect kept) or a placeholder.
func (s *screen) drawCover(x, y int32) {
	s.hero.poll(s.r)
	s.fill(track, &sdl.Rect{X: x, Y: y, W: coverBox, H: coverBox})
	if s.hero.tex != nil {
		s.r.Copy(s.hero.tex, nil, &sdl.Rect{X: x + (coverBox-s.hero.w)/2, Y: y + (coverBox-s.hero.h)/2, W: s.hero.w, H: s.hero.h})
	} else {
		s.text(s.f.Mid, "?", theme.Dimmed, x+coverBox/2-10, y+coverBox/2-24)
	}
	s.outline(x, y, coverBox, coverBox, theme.Dimmed, 60)
}

// ---- primitives ------------------------------------------------------------

// panel draws the hero card; active ones get a pulsing neon glow.
func (s *screen) panel(y int32, glow bool) {
	s.fill(theme.Panel, &sdl.Rect{X: margin, Y: y, W: screenW - 2*margin, H: heroH})
	if !glow {
		s.outline(margin, y, screenW-2*margin, heroH, theme.Dimmed, 70)
		return
	}
	p := s.pulse(90)
	for i, a := range []float64{0.9, 0.4, 0.18} {
		s.outline(int32(margin-i), y-int32(i), int32(screenW-2*margin+2*i), int32(heroH+2*i), theme.Neon, uint8(255*a*(0.55+0.45*p)))
	}
}

// bar draws a progress bar; shimmer adds a moving highlight over the fill.
func (s *screen) bar(x, y, w, height int, frac float64, shimmer bool) {
	h := int32(height)
	s.fill(track, &sdl.Rect{X: int32(x), Y: int32(y), W: int32(w), H: h})
	fw := int32(float64(w) * math.Max(0, math.Min(1, frac)))
	if fw <= 0 {
		return
	}
	s.fill(theme.Neon, &sdl.Rect{X: int32(x), Y: int32(y), W: fw, H: h})
	if !shimmer {
		return
	}
	// A soft diagonal-ish sweep, clipped to the filled part.
	const sw = 28
	sx := int32(x) + int32(s.frame*3)%(fw+sw) - sw
	for i := int32(0); i < sw; i++ {
		px := sx + i
		if px < int32(x) || px >= int32(x)+fw {
			continue
		}
		a := uint8(90 * math.Sin(math.Pi*float64(i)/sw))
		s.r.SetDrawColor(255, 255, 255, a)
		s.r.FillRect(&sdl.Rect{X: px, Y: int32(y), W: 1, H: h})
	}
	s.fill(sdl.Color{R: theme.Neon.R, G: theme.Neon.G, B: theme.Neon.B, A: 120}, &sdl.Rect{X: int32(x), Y: int32(y) + h - 2, W: fw, H: 2})
}

// icon draws the 14px status mark of a queue row at (x, y).
func (s *screen) icon(it download.Item, x, y int32, isActive bool) {
	switch {
	case isActive:
		a := uint8(120 + 135*s.pulse(40))
		s.r.SetDrawColor(theme.Neon.R, theme.Neon.G, theme.Neon.B, a)
		s.r.FillRect(&sdl.Rect{X: x + 2, Y: y + 2, W: 10, H: 10})
	case it.Status == download.Done && it.Skipped:
		s.check(x+7, y+7, 7, 2, theme.Dimmed)
	case it.Status == download.Done:
		s.check(x+7, y+7, 7, 2, theme.Neon)
	case it.Status == download.Failed:
		s.cross(x+7, y+7, 6, 2, theme.Pink)
	default:
		s.outline(x+1, y+1, 12, 12, theme.Dimmed, 200)
	}
}

// check draws a tick centred on (cx, cy); size is half its width.
func (s *screen) check(cx, cy, size, thick int32, c sdl.Color) {
	s.r.SetDrawColor(c.R, c.G, c.B, 255)
	a := [2]int32{cx - size, cy}
	b := [2]int32{cx - size/3, cy + size*2/3}
	d := [2]int32{cx + size, cy - size*2/3}
	for t := -thick / 2; t <= thick/2; t++ {
		s.r.DrawLine(a[0], a[1]+t, b[0], b[1]+t)
		s.r.DrawLine(b[0], b[1]+t, d[0], d[1]+t)
	}
}

// cross draws an X centred on (cx, cy).
func (s *screen) cross(cx, cy, size, thick int32, c sdl.Color) {
	s.r.SetDrawColor(c.R, c.G, c.B, 255)
	for t := -thick / 2; t <= thick/2; t++ {
		s.r.DrawLine(cx-size, cy-size+t, cx+size, cy+size+t)
		s.r.DrawLine(cx-size, cy+size+t, cx+size, cy-size+t)
	}
}

// chip draws a small button badge with its letter.
func (s *screen) chip(letter string, x, y int32) {
	s.fill(theme.Panel, &sdl.Rect{X: x, Y: y - 1, W: 18, H: 18})
	s.outline(x, y-1, 18, 18, theme.Dimmed, 160)
	s.text(s.f.Tiny, letter, white, x+4, y-1)
}

func (s *screen) outline(x, y, w, h int32, c sdl.Color, a uint8) {
	s.r.SetDrawColor(c.R, c.G, c.B, a)
	s.r.DrawRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
}

func (s *screen) fill(c sdl.Color, rc *sdl.Rect) {
	s.r.SetDrawColor(c.R, c.G, c.B, c.A)
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

// wrap2 splits str over at most two lines of maxW pixels; the second is
// shortened with an ellipsis if it still does not fit.
func (s *screen) wrap2(f *ttf.Font, str string, maxW int) (string, string) {
	if w, _, err := f.SizeUTF8(str); err != nil || w <= maxW {
		return str, ""
	}
	words := strings.Fields(str)
	line1 := ""
	i := 0
	for ; i < len(words); i++ {
		cand := strings.TrimSpace(line1 + " " + words[i])
		if w, _, err := f.SizeUTF8(cand); err == nil && w > maxW && line1 != "" {
			break
		}
		line1 = cand
	}
	if w, _, err := f.SizeUTF8(line1); err == nil && w > maxW { // one very long word
		return s.fit(f, line1, maxW), ""
	}
	return line1, s.fit(f, strings.Join(words[i:], " "), maxW)
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
