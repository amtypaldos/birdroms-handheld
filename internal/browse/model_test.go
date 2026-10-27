package browse

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/input"
)

// fakeSource is a canned Source that records every call it receives.
type fakeSource struct {
	playlists []api.Playlist
	platforms []string
	details   map[int]api.PlaylistDetail
	pages     map[string][]api.ROMPage // platform -> pages (index 0 = page 1)

	err   error // returned by every call when set
	calls []string
}

func (f *fakeSource) ListPlaylists() ([]api.Playlist, error) {
	f.calls = append(f.calls, "ListPlaylists")
	return f.playlists, f.err
}

func (f *fakeSource) ListPlatforms() ([]string, error) {
	f.calls = append(f.calls, "ListPlatforms")
	return f.platforms, f.err
}

func (f *fakeSource) GetPlaylist(id int) (api.PlaylistDetail, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetPlaylist(%d)", id))
	if f.err != nil {
		return api.PlaylistDetail{}, f.err
	}
	return f.details[id], nil
}

func (f *fakeSource) ListROMs(platform string, page int) (api.ROMPage, error) {
	f.calls = append(f.calls, fmt.Sprintf("ListROMs(%s,%d)", platform, page))
	if f.err != nil {
		return api.ROMPage{}, f.err
	}
	ps := f.pages[platform]
	if page < 1 || page > len(ps) {
		return api.ROMPage{}, nil
	}
	return ps[page-1], nil
}

func (f *fakeSource) callCount(call string) int {
	n := 0
	for _, c := range f.calls {
		if c == call {
			n++
		}
	}
	return n
}

func roms(platform string, firstID, n int) []api.ROM {
	out := make([]api.ROM, n)
	for i := range out {
		out[i] = api.ROM{ID: firstID + i, Title: fmt.Sprintf("%s %d", platform, firstID+i), Platform: platform}
	}
	return out
}

func link(s string) *string { return &s }

// pagesOf splits n ROMs of platform into pages of size per; every page but the
// last has a Next link.
func pagesOf(platform string, n, per int) []api.ROMPage {
	var out []api.ROMPage
	for start := 0; start < n; start += per {
		c := per
		if start+c > n {
			c = n - start
		}
		p := api.ROMPage{Count: n, Results: roms(platform, 1000+start, c)}
		if start+c < n {
			p.Next = link("http://x/next")
		}
		out = append(out, p)
	}
	return out
}

func ids(rs []api.ROM) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

// do presses b and runs the resulting effect to completion, failing on error.
func do(t *testing.T, m *Model, b input.Button) {
	t.Helper()
	if err := m.Run(m.Press(b)); err != nil {
		t.Fatalf("press %v: %v", b, err)
	}
}

func presses(t *testing.T, m *Model, bs ...input.Button) {
	t.Helper()
	for _, b := range bs {
		do(t, m, b)
	}
}

func newHome(t *testing.T, f *fakeSource) *Model {
	t.Helper()
	m := New(f)
	if err := m.Run(m.Start()); err != nil {
		t.Fatal(err)
	}
	return m
}

func standardSource() *fakeSource {
	return &fakeSource{
		playlists: []api.Playlist{
			{ID: 5, Name: "Road trip", ROMCount: 3},
			{ID: 6, Name: "Empty list", ROMCount: 0},
		},
		platforms: []string{"GBA", "SFC"},
		details: map[int]api.PlaylistDetail{
			5: {Playlist: api.Playlist{ID: 5, Name: "Road trip"}, ROMs: []api.ROM{{ID: 30}, {ID: 10}, {ID: 20}}},
			6: {Playlist: api.Playlist{ID: 6, Name: "Empty list"}},
		},
		pages: map[string][]api.ROMPage{
			"GBA": {{Count: 10, Results: roms("GBA", 100, 10)}},
			"SFC": {{Count: 0}},
		},
	}
}

func TestHomeLoadCallsOnlyListEndpoints(t *testing.T) {
	f := standardSource()
	newHome(t, f)
	got := map[string]int{}
	for _, c := range f.calls {
		got[c]++
	}
	want := map[string]int{"ListPlaylists": 1, "ListPlatforms": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want exactly %v (no GetPlaylist / ListROMs on home)", f.calls, want)
	}
}

func TestHomeRowsAreHeadersPlaylistsThenPlatforms(t *testing.T) {
	m := newHome(t, standardSource())
	v := m.View()
	if v.Mode != ModeHome {
		t.Fatalf("mode = %v", v.Mode)
	}
	want := []Row{
		{Kind: RowHeader, Text: "Playlists"},
		{Kind: RowPlaylist, Text: "Road trip", Count: 3},
		{Kind: RowPlaylist, Text: "Empty list", Count: 0},
		{Kind: RowHeader, Text: "Platforms"},
		{Kind: RowPlatform, Text: "GBA"},
		{Kind: RowPlatform, Text: "SFC"},
	}
	if !reflect.DeepEqual(v.Rows, want) {
		t.Fatalf("rows = %+v\nwant   %+v", v.Rows, want)
	}
	if v.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (first selectable row, not the header)", v.Cursor)
	}
}

func TestHomeUpDownSkipHeadersAndClamp(t *testing.T) {
	m := newHome(t, standardSource())
	cursors := func(bs ...input.Button) []int {
		var out []int
		for _, b := range bs {
			do(t, m, b)
			out = append(out, m.View().Cursor)
		}
		return out
	}
	// rows: 0 hdr, 1 Road trip, 2 Empty list, 3 hdr, 4 GBA, 5 SFC
	got := cursors(input.Up, input.Down, input.Down, input.Down, input.Down, input.Up)
	want := []int{1, 2, 4, 5, 5, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cursor trail = %v, want %v", got, want)
	}
}

func TestHomeLeftRightDoNothing(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	before := len(f.calls)
	for _, b := range []input.Button{input.Left, input.Right} {
		if e := m.Press(b); e.Kind != EffectNone {
			t.Fatalf("%v on home produced effect %+v", b, e)
		}
	}
	if m.View().Cursor != 1 || m.View().Mode != ModeHome || len(f.calls) != before {
		t.Fatalf("left/right changed state: %+v calls=%v", m.View(), f.calls)
	}
}

func TestOpenPlaylistCallsGetPlaylistOnceInServerOrder(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	do(t, m, input.A) // cursor starts on "Road trip"
	v := m.View()
	if v.Mode != ModeGrid || v.Title != "Road trip" {
		t.Fatalf("view = mode %v title %q", v.Mode, v.Title)
	}
	if got := ids(v.ROMs); !reflect.DeepEqual(got, []int{30, 10, 20}) {
		t.Fatalf("rom order = %v, want server order [30 10 20]", got)
	}
	if n := f.callCount("GetPlaylist(5)"); n != 1 {
		t.Fatalf("GetPlaylist calls = %d, want 1", n)
	}
	if v.Focus != 0 {
		t.Fatalf("focus = %d", v.Focus)
	}
}

func TestOpenPlatformCallsListROMsPageOne(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	presses(t, m, input.Down, input.Down, input.A) // -> GBA
	v := m.View()
	if v.Mode != ModeGrid || v.Title != "GBA" || len(v.ROMs) != 10 {
		t.Fatalf("view = mode %v title %q roms %d", v.Mode, v.Title, len(v.ROMs))
	}
	if n := f.callCount("ListROMs(GBA,1)"); n != 1 {
		t.Fatalf("calls = %v", f.calls)
	}
}

func TestSecondAWhileOpeningDoesNotRefetch(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	e := m.Press(input.A)
	if e.Kind == EffectNone {
		t.Fatal("first A produced no effect")
	}
	if e2 := m.Press(input.A); e2.Kind != EffectNone {
		t.Fatalf("second A while opening produced %+v", e2)
	}
	if err := m.Run(e); err != nil {
		t.Fatal(err)
	}
	if n := f.callCount("GetPlaylist(5)"); n != 1 {
		t.Fatalf("GetPlaylist calls = %d", n)
	}
}

func openGBA(t *testing.T, f *fakeSource) *Model {
	t.Helper()
	m := newHome(t, f)
	presses(t, m, input.Down, input.Down, input.A)
	if m.View().Mode != ModeGrid {
		t.Fatal("did not open grid")
	}
	return m
}

func TestGridMovesFourColumnsAndClamps(t *testing.T) {
	m := openGBA(t, standardSource()) // 10 ROMs
	focus := func(bs ...input.Button) []int {
		var out []int
		for _, b := range bs {
			do(t, m, b)
			out = append(out, m.View().Focus)
		}
		return out
	}
	got := focus(
		input.Left,  // 0 clamp
		input.Up,    // 0 clamp
		input.Right, // 1
		input.Right, // 2
		input.Down,  // 6
		input.Down,  // 10 -> clamp 9
		input.Down,  // 9
		input.Right, // 9 clamp
		input.Left,  // 8
		input.Up,    // 4
		input.Up,    // 0
		input.Up,    // 0
	)
	want := []int{0, 0, 1, 2, 6, 9, 9, 9, 8, 4, 0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("focus trail = %v, want %v", got, want)
	}
}

func TestSelectionTogglesAndKeepsAddOrder(t *testing.T) {
	m := openGBA(t, standardSource())
	presses(t, m, input.Right, input.Right, input.A)              // 102
	presses(t, m, input.Left, input.Left, input.A)                // 100
	presses(t, m, input.Right, input.Right, input.Right, input.A) // 103
	if got := ids(m.Selected()); !reflect.DeepEqual(got, []int{102, 100, 103}) {
		t.Fatalf("selected = %v, want add order [102 100 103]", got)
	}
	if v := m.View(); !v.Selected[102] || !v.Selected[100] || !v.Selected[103] || v.Selected[101] {
		t.Fatalf("view selection = %v", v.Selected)
	}
	presses(t, m, input.A)                                     // un-toggle 103
	presses(t, m, input.Left, input.Left, input.Left, input.A) // 100 off
	if got := ids(m.Selected()); !reflect.DeepEqual(got, []int{102}) {
		t.Fatalf("selected after toggle off = %v", got)
	}
}

func TestSelectionSurvivesBackAndReopen(t *testing.T) {
	m := openGBA(t, standardSource())
	presses(t, m, input.A) // select 100
	presses(t, m, input.B) // home
	if m.View().Mode != ModeHome {
		t.Fatal("B did not return home")
	}
	if got := ids(m.Selected()); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("selection lost on back: %v", got)
	}
	presses(t, m, input.A) // reopen GBA (cursor kept)
	if v := m.View(); !v.Selected[100] || v.Mode != ModeGrid {
		t.Fatalf("selection lost on reopen: %+v", v)
	}
}

func TestBackKeepsHomeCursor(t *testing.T) {
	m := newHome(t, standardSource())
	presses(t, m, input.Down, input.Down, input.Down, input.A, input.B) // SFC
	if v := m.View(); v.Mode != ModeHome || v.Cursor != 5 {
		t.Fatalf("after back: mode %v cursor %d, want home/5", v.Mode, v.Cursor)
	}
}

func TestExitButtons(t *testing.T) {
	t.Run("B on home exits", func(t *testing.T) {
		m := newHome(t, standardSource())
		do(t, m, input.B)
		if !m.Done() {
			t.Fatal("not done")
		}
	})
	t.Run("B in grid does not exit", func(t *testing.T) {
		m := openGBA(t, standardSource())
		do(t, m, input.B)
		if m.Done() {
			t.Fatal("done after B in grid")
		}
	})
	t.Run("Menu exits from home", func(t *testing.T) {
		m := newHome(t, standardSource())
		do(t, m, input.Menu)
		if !m.Done() {
			t.Fatal("not done")
		}
	})
	t.Run("Menu exits from grid", func(t *testing.T) {
		m := openGBA(t, standardSource())
		do(t, m, input.Menu)
		if !m.Done() {
			t.Fatal("not done")
		}
	})
	t.Run("Start is not a button here", func(t *testing.T) {
		m := openGBA(t, standardSource())
		before := m.View()
		if e := m.Press(input.Start); e.Kind != EffectNone || m.Done() {
			t.Fatalf("Start did something: %+v done=%v", e, m.Done())
		}
		if got := m.View(); got.Focus != before.Focus || len(got.Selected) != 0 {
			t.Fatalf("Start changed state")
		}
	})
}

// ---- lazy paging -----------------------------------------------------------

func pagedSource(total, per int) *fakeSource {
	f := standardSource()
	f.platforms = []string{"GBA"}
	f.playlists = nil
	f.pages = map[string][]api.ROMPage{"GBA": pagesOf("GBA", total, per)}
	return f
}

func openFirstPlatform(t *testing.T, f *fakeSource) *Model {
	t.Helper()
	m := newHome(t, f)
	do(t, m, input.A)
	if v := m.View(); v.Mode != ModeGrid {
		t.Fatalf("not in grid: %+v", v)
	}
	return m
}

func TestLazyPageRequestedWithinLastFourOnly(t *testing.T) {
	f := pagedSource(36, 12)
	m := openFirstPlatform(t, f)
	if len(m.View().ROMs) != 12 {
		t.Fatalf("loaded = %d, want only page 1 (12)", len(m.View().ROMs))
	}
	// Walk focus 0 -> 7 with Right: still outside the last four (indexes 8..11).
	for i := 0; i < 7; i++ {
		do(t, m, input.Right)
	}
	if n := f.callCount("ListROMs(GBA,2)"); n != 0 {
		t.Fatalf("page 2 requested too early (focus 7 of 12): %v", f.calls)
	}
	do(t, m, input.Right) // focus 8 = within last 4
	if n := f.callCount("ListROMs(GBA,2)"); n != 1 {
		t.Fatalf("page 2 calls = %d, want 1: %v", n, f.calls)
	}
	if len(m.View().ROMs) != 24 {
		t.Fatalf("loaded = %d, want 24 after append", len(m.View().ROMs))
	}
	if got := ids(m.View().ROMs[10:14]); !reflect.DeepEqual(got, []int{1010, 1011, 1012, 1013}) {
		t.Fatalf("append order wrong: %v", got)
	}
}

func TestLazyPageNeverRequestedTwice(t *testing.T) {
	f := pagedSource(36, 12)
	m := openFirstPlatform(t, f)
	presses(t, m, input.Down, input.Down) // 8
	// Pressing the same page's trigger zone again, before and after it loads.
	e := m.Press(input.Right) // 9
	_ = e
	presses(t, m, input.Right, input.Left, input.Right)
	if n := f.callCount("ListROMs(GBA,2)"); n != 1 {
		t.Fatalf("page 2 calls = %d: %v", n, f.calls)
	}
}

func TestLazyPageInFlightIsNotDuplicated(t *testing.T) {
	f := pagedSource(36, 12)
	m := openFirstPlatform(t, f)
	presses(t, m, input.Down) // 4
	m.Press(input.Down)       // 8: marks page 2 requested, effect not run yet
	e := m.Press(input.Right) // 9: page 2 is already pending
	if e.Kind != EffectNone {
		t.Fatalf("second request for in-flight page: %+v", e)
	}
}

func TestLazyPagesChainThroughToTheEnd(t *testing.T) {
	f := pagedSource(30, 12) // pages of 12, 12, 6
	m := openFirstPlatform(t, f)
	for i := 0; i < 40; i++ {
		do(t, m, input.Down)
	}
	if got := len(m.View().ROMs); got != 30 {
		t.Fatalf("loaded = %d, want all 30", got)
	}
	want := []string{"ListPlaylists", "ListPlatforms", "ListROMs(GBA,1)", "ListROMs(GBA,2)", "ListROMs(GBA,3)"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("calls = %v, want %v (no page 4)", f.calls, want)
	}
}

func TestLazyPageRequestedOnOpenWhenFirstPageIsShort(t *testing.T) {
	f := pagedSource(10, 3) // pages of 3: every ROM of page 1 is within the last 4
	m := openFirstPlatform(t, f)
	if n := f.callCount("ListROMs(GBA,2)"); n != 1 {
		t.Fatalf("calls = %v, want page 2 fetched at open", f.calls)
	}
	// Opening with focus 0 and 6 loaded ROMs: still within last 4? 0 >= 6-4 is false.
	if len(m.View().ROMs) < 6 {
		t.Fatalf("loaded %d", len(m.View().ROMs))
	}
}

func TestStaleLazyPageIsDiscardedAfterLeavingGrid(t *testing.T) {
	f := pagedSource(36, 12)
	f.platforms = []string{"GBA", "SFC"}
	f.pages["SFC"] = []api.ROMPage{{Count: 5, Results: roms("SFC", 2000, 5)}}
	m := openFirstPlatform(t, f)
	presses(t, m, input.Down)    // 4
	stale := m.Press(input.Down) // 8 -> page-2 effect, not yet run
	if stale.Kind == EffectNone {
		t.Fatal("expected a page effect")
	}
	presses(t, m, input.B, input.Down, input.A) // home -> SFC grid
	if err := m.Run(stale); err != nil {
		t.Fatal(err)
	}
	if got := ids(m.View().ROMs); !reflect.DeepEqual(got, []int{2000, 2001, 2002, 2003, 2004}) {
		t.Fatalf("stale GBA page leaked into SFC grid: %v", got)
	}
}

func TestPlaylistGridDoesNotPage(t *testing.T) {
	f := standardSource()
	f.details[5] = api.PlaylistDetail{Playlist: api.Playlist{ID: 5, Name: "Road trip"}, ROMs: roms("GBA", 1, 12)}
	m := newHome(t, f)
	do(t, m, input.A)
	for i := 0; i < 10; i++ {
		do(t, m, input.Down)
	}
	for _, c := range f.calls {
		if len(c) >= 8 && c[:8] == "ListROMs" {
			t.Fatalf("playlist grid called %s: %v", c, f.calls)
		}
	}
	if len(m.View().ROMs) != 12 {
		t.Fatalf("roms = %d", len(m.View().ROMs))
	}
}

func TestEmptyPageStopsPaging(t *testing.T) {
	f := pagedSource(0, 12)
	f.pages["GBA"] = []api.ROMPage{{Count: 0}}
	m := openFirstPlatform(t, f)
	for _, b := range []input.Button{input.Down, input.Right, input.Up} {
		do(t, m, b)
	}
	if n := f.callCount("ListROMs(GBA,2)"); n != 0 {
		t.Fatalf("requested page 2 after empty page: %v", f.calls)
	}
}

func TestNonEmptyLastPageStopsPaging(t *testing.T) {
	f := pagedSource(12, 12) // single page, Next == nil
	m := openFirstPlatform(t, f)
	for i := 0; i < 15; i++ {
		do(t, m, input.Down)
	}
	if n := f.callCount("ListROMs(GBA,2)"); n != 0 {
		t.Fatalf("requested a page past Next==nil: %v", f.calls)
	}
}

func TestPageWithNextButNoResultsDoesNotLoopForever(t *testing.T) {
	f := pagedSource(0, 12)
	f.pages["GBA"] = []api.ROMPage{{Count: 5, Next: link("http://x/n")}} // pathological server
	m := openFirstPlatform(t, f)
	if n := f.callCount("ListROMs(GBA,2)"); n != 0 {
		t.Fatalf("followed Next from an empty page: %v", f.calls)
	}
	_ = m
}

// ---- empty states ----------------------------------------------------------

func TestEmptyHome(t *testing.T) {
	f := &fakeSource{}
	m := newHome(t, f)
	v := m.View()
	for _, r := range v.Rows {
		if r.Kind == RowPlaylist || r.Kind == RowPlatform {
			t.Fatalf("selectable row in empty home: %+v", r)
		}
	}
	want := []Row{
		{Kind: RowHeader, Text: "Playlists"},
		{Kind: RowNote, Text: "No playlists"},
		{Kind: RowHeader, Text: "Platforms"},
		{Kind: RowNote, Text: "No platforms"},
	}
	if !reflect.DeepEqual(v.Rows, want) {
		t.Fatalf("rows = %+v", v.Rows)
	}
	before := len(f.calls)
	for _, b := range []input.Button{input.Up, input.Down, input.A, input.Left, input.Right} {
		if e := m.Press(b); e.Kind != EffectNone {
			t.Fatalf("%v on empty home gave %+v", b, e)
		}
	}
	if m.View().Mode != ModeHome || len(f.calls) != before {
		t.Fatalf("empty home navigated or called: %v", f.calls)
	}
	do(t, m, input.B)
	if !m.Done() {
		t.Fatal("B should still exit")
	}
}

func TestHomeWithOnlyPlatformsShowsPlaylistsNote(t *testing.T) {
	f := &fakeSource{platforms: []string{"GBA"}}
	m := newHome(t, f)
	v := m.View()
	want := []Row{
		{Kind: RowHeader, Text: "Playlists"},
		{Kind: RowNote, Text: "No playlists"},
		{Kind: RowHeader, Text: "Platforms"},
		{Kind: RowPlatform, Text: "GBA"},
	}
	if !reflect.DeepEqual(v.Rows, want) || v.Cursor != 3 {
		t.Fatalf("rows = %+v cursor %d", v.Rows, v.Cursor)
	}
}

func TestEmptyPlatformGrid(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	presses(t, m, input.Down, input.Down, input.Down, input.A) // SFC: zero ROMs
	v := m.View()
	if v.Mode != ModeGrid || len(v.ROMs) != 0 || v.Empty != "Nothing here" {
		t.Fatalf("view = %+v", v)
	}
	before := len(f.calls)
	presses(t, m, input.Down, input.Right, input.A) // nothing to move or toggle
	if len(m.Selected()) != 0 || v.Focus != 0 || len(f.calls) != before {
		t.Fatalf("empty grid acted: selected=%v calls=%v", m.Selected(), f.calls)
	}
	do(t, m, input.B)
	if m.View().Mode != ModeHome {
		t.Fatal("B from empty grid should go home")
	}
}

func TestEmptyPlaylistGrid(t *testing.T) {
	m := newHome(t, standardSource())
	presses(t, m, input.Down, input.A) // "Empty list"
	if v := m.View(); v.Mode != ModeGrid || len(v.ROMs) != 0 || v.Empty != "Nothing here" {
		t.Fatalf("view = %+v", v)
	}
}

func TestNonEmptyGridHasNoEmptyMessage(t *testing.T) {
	m := openGBA(t, standardSource())
	if m.View().Empty != "" {
		t.Fatalf("empty = %q", m.View().Empty)
	}
}

// ---- errors ----------------------------------------------------------------

func TestUnauthorizedIsReturnedAndNotRetried(t *testing.T) {
	wrapped := fmt.Errorf("GET /x: %w", api.ErrUnauthorized)

	t.Run("home load", func(t *testing.T) {
		f := &fakeSource{err: wrapped}
		m := New(f)
		err := m.Run(m.Start())
		if !errors.Is(err, api.ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
		if !m.View().Unauthorized {
			t.Fatal("View().Unauthorized not set")
		}
		if n := len(f.calls); n > 2 {
			t.Fatalf("retried: %v", f.calls)
		}
		if e := m.Press(input.A); e.Kind != EffectNone {
			t.Fatalf("model kept working after 401: %+v", e)
		}
	})

	t.Run("open playlist", func(t *testing.T) {
		f := standardSource()
		m := newHome(t, f)
		f.err = wrapped
		before := len(f.calls)
		err := m.Run(m.Press(input.A))
		if !errors.Is(err, api.ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
		if len(f.calls)-before != 1 {
			t.Fatalf("calls after 401 = %v", f.calls[before:])
		}
		if !m.View().Unauthorized {
			t.Fatal("Unauthorized not set")
		}
	})

	t.Run("open platform", func(t *testing.T) {
		f := standardSource()
		m := newHome(t, f)
		presses(t, m, input.Down, input.Down)
		f.err = wrapped
		if err := m.Run(m.Press(input.A)); !errors.Is(err, api.ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("lazy page", func(t *testing.T) {
		f := pagedSource(36, 12)
		m := openFirstPlatform(t, f)
		presses(t, m, input.Down)
		f.err = wrapped
		if err := m.Run(m.Press(input.Down)); !errors.Is(err, api.ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
		before := len(f.calls)
		m.Run(m.Press(input.Right))
		if len(f.calls) != before {
			t.Fatalf("lazy page retried after 401: %v", f.calls[before:])
		}
	})
}

func TestOrdinaryErrorKeepsModelUsableAndRetries(t *testing.T) {
	f := standardSource()
	m := newHome(t, f)
	f.err = errors.New("boom")
	err := m.Run(m.Press(input.A))
	if err == nil || errors.Is(err, api.ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
	v := m.View()
	if v.Mode != ModeHome || v.Err == "" || v.Unauthorized {
		t.Fatalf("view after error = %+v", v)
	}
	f.err = nil
	do(t, m, input.A) // retry works
	if v := m.View(); v.Mode != ModeGrid || v.Err != "" {
		t.Fatalf("retry view = %+v", v)
	}
}

func TestFailedLazyPageCanBeRetried(t *testing.T) {
	f := pagedSource(36, 12)
	m := openFirstPlatform(t, f)
	presses(t, m, input.Down)
	f.err = errors.New("flaky")
	if err := m.Run(m.Press(input.Down)); err == nil { // 8: page 2 fails
		t.Fatal("expected error")
	}
	f.err = nil
	do(t, m, input.Right) // 9: retry
	if got := len(m.View().ROMs); got != 24 {
		t.Fatalf("loaded = %d, want 24 after retry; calls %v", got, f.calls)
	}
}

func TestSelectedIsACopy(t *testing.T) {
	m := openGBA(t, standardSource())
	do(t, m, input.A)
	s := m.Selected()
	s[0].ID = -1
	if m.Selected()[0].ID != 100 {
		t.Fatal("Selected() leaked internal slice")
	}
}

func TestFailedHomeLoadCanBeRetriedWithA(t *testing.T) {
	f := standardSource()
	f.err = errors.New("offline")
	m := New(f)
	if err := m.Run(m.Start()); err == nil {
		t.Fatal("expected error")
	}
	if v := m.View(); v.Err == "" || v.Loading || v.Cursor != -1 {
		t.Fatalf("view after failed load = %+v", v)
	}
	f.err = nil
	do(t, m, input.A) // retry
	if v := m.View(); v.Err != "" || v.Cursor != 1 || len(v.Rows) != 6 {
		t.Fatalf("view after retry = %+v", v)
	}
	// Once loaded, A on a row opens it rather than reloading home.
	do(t, m, input.A)
	if m.View().Mode != ModeGrid {
		t.Fatal("A after a successful load should open the row")
	}
}
