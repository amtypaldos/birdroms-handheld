// Package browse is the SDL-free state model behind the handheld browse
// screens: a home list (playlists, then platforms) and a 4-column cover grid
// of ROMs. It records which ROMs the user selected; it never downloads.
//
// The model is split so the SDL thread never waits on the network:
//
//	e := m.Press(button) // instant: moves focus / selection, returns what to fetch
//	go m.Run(e)          // slow: does the HTTP call, applies the result
//
// Press and View are cheap and safe to call every frame; Run blocks on the
// Source and may be called from any goroutine. Results that arrive after the
// user has left the screen they were for are discarded.
package browse

import (
	"errors"
	"sync"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/input"
)

// Cols is the grid width in cells.
const Cols = 4

// lazyZone is how close to the end of the loaded ROMs the focus must get
// before the next page is requested.
const lazyZone = 4

// Source is the part of *api.Client the model needs.
type Source interface {
	ListPlatforms() ([]string, error)
	ListROMs(platform string, page int) (api.ROMPage, error)
	ListPlaylists() ([]api.Playlist, error)
	GetPlaylist(id int) (api.PlaylistDetail, error)
}

// Mode is which screen is showing.
type Mode int

const (
	ModeHome Mode = iota
	ModeGrid
)

// RowKind classifies a home row. Only RowPlaylist and RowPlatform are selectable.
type RowKind int

const (
	RowHeader RowKind = iota
	RowPlaylist
	RowPlatform
	RowNote // plain text such as "No playlists"
)

// Row is one line of the home list.
type Row struct {
	Kind  RowKind
	Text  string
	Count int // ROM count, playlists only
}

// Copy shown for empty states.
const (
	TextNoPlaylists = "No playlists"
	TextNoPlatforms = "No platforms"
	TextNothingHere = "Nothing here"
)

// EffectKind says what network work a button press needs.
type EffectKind int

const (
	EffectNone EffectKind = iota
	EffectLoadHome
	EffectOpenPlaylist
	EffectOpenPlatform
	EffectNextPage
)

// Effect is a unit of network work returned by Start and Press and executed by
// Run. The zero value does nothing.
type Effect struct {
	Kind     EffectKind
	ID       int    // playlist id
	Platform string // platform name (open / next page)
	Name     string // playlist name
	Page     int    // page to fetch (platform grids)
	gen      int    // grid generation the effect belongs to
}

// View is an immutable snapshot of what to draw.
type View struct {
	Mode Mode

	// Home.
	Rows   []Row
	Cursor int // index into Rows of the focused row, -1 when nothing is selectable

	// Grid.
	Title    string
	ROMs     []api.ROM // loaded so far; do not modify
	Focus    int       // index into ROMs
	Selected map[int]bool
	Empty    string // non-empty when the grid has nothing to show
	// AllSelected is true when the grid is fully loaded (no pages left) and
	// every ROM in it is selected: Y would deselect them.
	AllSelected bool
	// SelectingAll is true while Y is fetching the remaining pages.
	SelectingAll bool

	// Loading is true while an open (not a background page) is in flight.
	Loading bool
	// Err is a short message for the last failed non-auth request, "" if none.
	Err string
	// Unauthorized is set once any call reported a revoked token. The model
	// stops accepting input except exit.
	Unauthorized bool
}

// Model is the browse state machine. Create with New.
type Model struct {
	src Source

	mu sync.Mutex

	done         bool
	unauthorized bool
	errMsg       string
	loading      bool // an open / home load is in flight
	homeLoaded   bool

	playlists []api.Playlist
	platforms []string
	cursor    int // index into rows (see rows())

	mode Mode

	// Grid state; valid in ModeGrid (and while an open is pending).
	gen        int
	title      string
	playlistID int // >0 for a playlist grid
	platform   string
	items      []api.ROM
	focus      int
	loadedPage int
	hasNext    bool
	asked      int  // highest page requested (>= loadedPage)
	selectAll  bool // Y is fetching the remaining pages to select the whole platform

	selected []api.ROM
	selIDs   map[int]bool
}

// New returns a model that reads from src. Call Run(m.Start()) to load home.
func New(src Source) *Model {
	return &Model{src: src, selIDs: map[int]bool{}, cursor: -1}
}

// Start returns the effect that loads the home lists.
func (m *Model) Start() Effect {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loading = true
	return Effect{Kind: EffectLoadHome}
}

// Done reports whether the user asked to leave the browse screen.
func (m *Model) Done() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done
}

// Selected returns the selected ROMs in the order they were added.
func (m *Model) Selected() []api.ROM {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]api.ROM(nil), m.selected...)
}

// rows builds the home list. Caller holds the lock.
func (m *Model) rows() []Row {
	out := []Row{{Kind: RowHeader, Text: "Playlists"}}
	if len(m.playlists) == 0 {
		out = append(out, Row{Kind: RowNote, Text: TextNoPlaylists})
	}
	for _, p := range m.playlists {
		out = append(out, Row{Kind: RowPlaylist, Text: p.Name, Count: p.ROMCount})
	}
	out = append(out, Row{Kind: RowHeader, Text: "Platforms"})
	if len(m.platforms) == 0 {
		out = append(out, Row{Kind: RowNote, Text: TextNoPlatforms})
	}
	for _, p := range m.platforms {
		out = append(out, Row{Kind: RowPlatform, Text: p})
	}
	return out
}

func selectable(r Row) bool { return r.Kind == RowPlaylist || r.Kind == RowPlatform }

// firstSelectable returns the first selectable row index, or -1.
func firstSelectable(rows []Row) int {
	for i, r := range rows {
		if selectable(r) {
			return i
		}
	}
	return -1
}

// View returns a snapshot for drawing.
func (m *Model) View() View {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := View{
		Mode:         m.mode,
		Loading:      m.loading,
		Err:          m.errMsg,
		Unauthorized: m.unauthorized,
		Cursor:       -1,
	}
	if m.mode == ModeHome {
		v.Rows = m.rows()
		v.Cursor = m.cursor
		return v
	}
	v.Title = m.title
	v.ROMs = m.items
	v.Focus = m.focus
	v.SelectingAll = m.selectAll
	v.AllSelected = m.allSelected()
	v.Selected = make(map[int]bool, len(m.selIDs))
	for id := range m.selIDs {
		v.Selected[id] = true
	}
	if len(m.items) == 0 && !m.loading {
		v.Empty = TextNothingHere
	}
	return v
}

// Press applies a button. Everything it does is instant; if network work is
// needed it is returned for Run.
func (m *Model) Press(b input.Button) Effect {
	m.mu.Lock()
	defer m.mu.Unlock()

	if b == input.Menu {
		m.done = true
		return Effect{}
	}
	if m.unauthorized {
		if b == input.B {
			m.done = true
		}
		return Effect{}
	}
	if m.mode == ModeHome {
		return m.pressHome(b)
	}
	return m.pressGrid(b)
}

func (m *Model) pressHome(b input.Button) Effect {
	if b == input.B {
		m.done = true
		return Effect{}
	}
	if m.loading {
		return Effect{}
	}
	if !m.homeLoaded { // the first load failed: A retries it
		if b == input.A {
			m.loading = true
			return Effect{Kind: EffectLoadHome}
		}
		return Effect{}
	}
	rows := m.rows()
	switch b {
	case input.Up:
		for i := m.cursor - 1; i >= 0; i-- {
			if selectable(rows[i]) {
				m.cursor = i
				break
			}
		}
	case input.Down:
		for i := m.cursor + 1; i < len(rows); i++ {
			if selectable(rows[i]) {
				m.cursor = i
				break
			}
		}
	case input.A:
		if m.cursor < 0 || m.cursor >= len(rows) || !selectable(rows[m.cursor]) {
			return Effect{}
		}
		r := rows[m.cursor]
		m.loading = true
		m.gen++
		if r.Kind == RowPlaylist {
			p := m.playlists[m.cursor-1] // row 0 is the header; empty case has no playlist rows
			return Effect{Kind: EffectOpenPlaylist, ID: p.ID, Name: p.Name, gen: m.gen}
		}
		return Effect{Kind: EffectOpenPlatform, Platform: r.Text, Page: 1, gen: m.gen}
	}
	return Effect{}
}

func (m *Model) pressGrid(b input.Button) Effect {
	n := len(m.items)
	switch b {
	case input.B:
		m.mode = ModeHome
		m.gen++ // invalidates in-flight page fetches for this grid
		m.selectAll = false
		m.items = nil
		m.loading = false
		return Effect{}
	case input.A:
		if n == 0 {
			return Effect{}
		}
		m.toggle(m.items[m.focus])
		return Effect{}
	case input.Y:
		return m.pressSelectAll()
	case input.Left:
		m.focus = clamp(m.focus-1, n)
	case input.Right:
		m.focus = clamp(m.focus+1, n)
	case input.Up:
		m.focus = clamp(m.focus-Cols, n)
	case input.Down:
		m.focus = clamp(m.focus+Cols, n)
	default:
		return Effect{}
	}
	return m.lazy()
}

// allSelected reports whether the grid is fully loaded and every ROM in it is
// selected. Caller holds the lock.
func (m *Model) allSelected() bool {
	if m.mode != ModeGrid || len(m.items) == 0 || m.hasNext {
		return false
	}
	for _, r := range m.items {
		if !m.selIDs[r.ID] {
			return false
		}
	}
	return true
}

// pressSelectAll is Y: select every ROM of the grid, or deselect them when they
// all are selected. On a platform with more pages it starts fetching them and
// selects once the last one lands. Caller holds the lock.
func (m *Model) pressSelectAll() Effect {
	if len(m.items) == 0 || m.selectAll {
		return Effect{}
	}
	if m.allSelected() {
		m.deselectItems()
		return Effect{}
	}
	if !m.hasNext {
		m.selectItems()
		return Effect{}
	}
	m.selectAll = true
	m.errMsg = ""
	if m.asked > m.loadedPage {
		return Effect{} // a page is already in flight; its arrival continues the job
	}
	m.asked = m.loadedPage + 1
	return Effect{Kind: EffectNextPage, Platform: m.platform, Page: m.asked, gen: m.gen}
}

// selectItems appends the grid's unselected ROMs in grid order.
func (m *Model) selectItems() {
	for _, r := range m.items {
		if !m.selIDs[r.ID] {
			m.toggle(r)
		}
	}
}

// deselectItems removes the grid's ROMs, leaving the rest of the selection.
func (m *Model) deselectItems() {
	for _, r := range m.items {
		if m.selIDs[r.ID] {
			m.toggle(r)
		}
	}
}

func clamp(i, n int) int {
	if i >= n {
		i = n - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

func (m *Model) toggle(r api.ROM) {
	if m.selIDs[r.ID] {
		delete(m.selIDs, r.ID)
		for i, s := range m.selected {
			if s.ID == r.ID {
				m.selected = append(m.selected[:i], m.selected[i+1:]...)
				break
			}
		}
		return
	}
	m.selIDs[r.ID] = true
	m.selected = append(m.selected, r)
}

// lazy returns the next-page effect when the focus is within the last lazyZone
// loaded ROMs, a next page exists and it has not been asked for. It marks the
// page as asked. Caller holds the lock.
func (m *Model) lazy() Effect {
	if m.mode != ModeGrid || m.unauthorized || m.playlistID != 0 || !m.hasNext {
		return Effect{}
	}
	if len(m.items) == 0 || m.focus < len(m.items)-lazyZone {
		return Effect{}
	}
	if m.asked > m.loadedPage {
		return Effect{} // already in flight
	}
	m.asked = m.loadedPage + 1
	return Effect{Kind: EffectNextPage, Platform: m.platform, Page: m.asked, gen: m.gen}
}

// Run executes e: it calls the Source (without holding the model lock) and
// applies the result. After a page lands it keeps going if the focus is still
// near the end. It returns the Source's error; api.ErrUnauthorized (checked
// with errors.Is) also sets View().Unauthorized and is never retried.
func (m *Model) Run(e Effect) error {
	for e.Kind != EffectNone {
		next, err := m.run(e)
		if err != nil {
			return err
		}
		e = next
	}
	return nil
}

func (m *Model) run(e Effect) (Effect, error) {
	switch e.Kind {
	case EffectLoadHome:
		pls, err := m.src.ListPlaylists()
		if err != nil {
			return Effect{}, m.fail(e, err)
		}
		pfs, err := m.src.ListPlatforms()
		if err != nil {
			return Effect{}, m.fail(e, err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.playlists, m.platforms = pls, pfs
		m.homeLoaded = true
		m.loading = false
		m.errMsg = ""
		m.cursor = firstSelectable(m.rows())
		return Effect{}, nil

	case EffectOpenPlaylist:
		d, err := m.src.GetPlaylist(e.ID)
		if err != nil {
			return Effect{}, m.fail(e, err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if e.gen != m.gen {
			return Effect{}, nil
		}
		m.openGrid(e.Name, e.ID, "", d.ROMs, 1, false)
		return Effect{}, nil

	case EffectOpenPlatform:
		p, err := m.src.ListROMs(e.Platform, e.Page)
		if err != nil {
			return Effect{}, m.fail(e, err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if e.gen != m.gen {
			return Effect{}, nil
		}
		m.openGrid(e.Platform, 0, e.Platform, p.Results, e.Page, p.Next != nil && len(p.Results) > 0)
		return m.lazy(), nil

	case EffectNextPage:
		p, err := m.src.ListROMs(e.Platform, e.Page)
		if err != nil {
			return Effect{}, m.fail(e, err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if e.gen != m.gen || m.mode != ModeGrid {
			return Effect{}, nil
		}
		m.items = append(m.items[:len(m.items):len(m.items)], p.Results...)
		m.loadedPage = e.Page
		m.hasNext = p.Next != nil && len(p.Results) > 0
		m.errMsg = ""
		if m.selectAll {
			if m.hasNext {
				m.asked = m.loadedPage + 1
				return Effect{Kind: EffectNextPage, Platform: m.platform, Page: m.asked, gen: m.gen}, nil
			}
			m.selectAll = false
			m.selectItems()
			return Effect{}, nil
		}
		return m.lazy(), nil
	}
	return Effect{}, nil
}

// openGrid switches to the grid for a freshly fetched first page. Caller holds the lock.
func (m *Model) openGrid(title string, playlistID int, platform string, results []api.ROM, page int, hasNext bool) {
	m.mode = ModeGrid
	m.loading = false
	m.errMsg = ""
	m.title = title
	m.playlistID = playlistID
	m.platform = platform
	m.items = append([]api.ROM(nil), results...)
	m.focus = 0
	m.loadedPage = page
	m.asked = page
	m.hasNext = hasNext
	m.selectAll = false
}

// fail records a failed effect and returns err. A 401 locks the model; any
// other error leaves it usable and lets the same action be retried.
func (m *Model) fail(e Effect, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.Kind == EffectNextPage {
		if e.gen == m.gen {
			m.selectAll = false // a failed page ends select-all; Y can start it again
			if m.asked == e.Page {
				m.asked = m.loadedPage // let the next focus move retry
			}
		}
	} else if e.Kind == EffectLoadHome || e.gen == m.gen {
		m.loading = false
	}
	if errors.Is(err, api.ErrUnauthorized) {
		m.unauthorized = true
		m.errMsg = ""
		return err
	}
	m.errMsg = "Could not load. Check connection."
	return err
}
