package browse

import (
	"bytes"
	"image"
	"image/png"
	"reflect"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

func TestCoverCacheSizeIs32(t *testing.T) {
	if CoverCacheSize != 32 {
		t.Fatalf("CoverCacheSize = %d, want 32", CoverCacheSize)
	}
}

func TestCache33rdDistinctDropsTheOldest(t *testing.T) {
	var evicted []int
	c := NewCache[string](CoverCacheSize, func(id int, _ string) { evicted = append(evicted, id) })
	for id := 1; id <= 32; id++ {
		c.Put(id, "v")
	}
	if len(evicted) != 0 || c.Len() != 32 {
		t.Fatalf("evicted %v len %d before capacity exceeded", evicted, c.Len())
	}
	c.Put(33, "v")
	if !reflect.DeepEqual(evicted, []int{1}) {
		t.Fatalf("evicted = %v, want [1]", evicted)
	}
	if _, ok := c.Get(1); ok {
		t.Fatal("oldest entry still present")
	}
	if _, ok := c.Get(33); !ok || c.Len() != 32 {
		t.Fatalf("len = %d", c.Len())
	}
}

func TestCacheGetRefreshesRecency(t *testing.T) {
	var evicted []int
	c := NewCache[int](3, func(id int, _ int) { evicted = append(evicted, id) })
	c.Put(1, 1)
	c.Put(2, 2)
	c.Put(3, 3)
	c.Get(1) // 1 is now newest; 2 is oldest
	c.Put(4, 4)
	if !reflect.DeepEqual(evicted, []int{2}) {
		t.Fatalf("evicted = %v, want [2]", evicted)
	}
}

func TestCachePutExistingReplacesAndReleasesOldValue(t *testing.T) {
	type ev struct{ id, v int }
	var evicted []ev
	c := NewCache[int](2, func(id int, v int) { evicted = append(evicted, ev{id, v}) })
	c.Put(1, 10)
	c.Put(2, 20)
	c.Put(1, 11) // replace: old value released, nothing else dropped
	if !reflect.DeepEqual(evicted, []ev{{1, 10}}) || c.Len() != 2 {
		t.Fatalf("evicted = %v len %d", evicted, c.Len())
	}
	if v, _ := c.Get(1); v != 11 {
		t.Fatalf("v = %d", v)
	}
	c.Put(3, 30) // 2 is oldest now (1 was refreshed by Put and Get)
	if evicted[len(evicted)-1] != (ev{2, 20}) {
		t.Fatalf("evicted = %v", evicted)
	}
}

func TestCacheClearReleasesEverything(t *testing.T) {
	n := 0
	c := NewCache[int](4, func(int, int) { n++ })
	c.Put(1, 1)
	c.Put(2, 2)
	c.Clear()
	if n != 2 || c.Len() != 0 {
		t.Fatalf("released %d, len %d", n, c.Len())
	}
}

func TestCacheNilOnEvictIsFine(t *testing.T) {
	c := NewCache[int](1, nil)
	c.Put(1, 1)
	c.Put(2, 2)
	if c.Len() != 1 {
		t.Fatal(c.Len())
	}
}

// ---- visibility policy -----------------------------------------------------

func TestScrollRowKeepsFocusVisible(t *testing.T) {
	cases := []struct{ focusRow, scroll, want int }{
		{0, 0, 0},
		{2, 0, 0},  // last visible row (3 rows: 0..2)
		{3, 0, 1},  // one past the bottom: scroll down by one
		{10, 0, 8}, // big jump: focus lands on the last visible row
		{1, 5, 1},  // above the window: scroll up to it
		{6, 5, 5},  // inside window: unchanged
	}
	for _, c := range cases {
		if got := ScrollRow(c.focusRow, c.scroll); got != c.want {
			t.Errorf("ScrollRow(%d,%d) = %d, want %d", c.focusRow, c.scroll, got, c.want)
		}
	}
}

func TestVisibleRangeClampsToLoaded(t *testing.T) {
	cases := []struct{ scrollRow, total, first, end int }{
		{0, 100, 0, 12},
		{2, 100, 8, 20},
		{0, 5, 0, 5},
		{1, 5, 4, 5},
		{0, 0, 0, 0},
	}
	for _, c := range cases {
		first, end := VisibleRange(c.scrollRow, c.total)
		if first != c.first || end != c.end {
			t.Errorf("VisibleRange(%d,%d) = %d,%d want %d,%d", c.scrollRow, c.total, first, end, c.first, c.end)
		}
	}
}

func TestWantCoversOnlyOnScreenWithURL(t *testing.T) {
	list := roms("GBA", 1, 30)
	for i := range list {
		if i%2 == 0 {
			list[i].CoverURL = link("http://x/c.jpg")
		}
	}
	got := WantCovers(list, 4, 8) // indexes 4..7
	if want := []int{5, 7}; !reflect.DeepEqual(ids(got), want) {
		// ids are 1-based: index 4 -> id 5 (has URL), 5 -> 6 (nil), 6 -> 7 (URL), 7 -> 8 (nil)
		t.Fatalf("WantCovers = %v, want ids %v", ids(got), want)
	}
	if n := len(WantCovers(list, 12, 99)); n != 9 { // clamped to 30; indexes 12..29 even have URLs
		t.Fatalf("clamped len = %d", n)
	}
	var none []api.ROM
	if len(WantCovers(none, 0, 12)) != 0 {
		t.Fatal("nil list")
	}
}

// ---- cover scaling ---------------------------------------------------------

func TestScaleToFitShrinksKeepingAspect(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 600, 300))
	out := ScaleToFit(src, 120, 120)
	if b := out.Bounds(); b.Dx() != 120 || b.Dy() != 60 {
		t.Fatalf("size = %v, want 120x60", b)
	}
}

func TestScaleToFitNeverUpscales(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 40, 30))
	if out := ScaleToFit(src, 120, 120); out.Bounds().Dx() != 40 || out.Bounds().Dy() != 30 {
		t.Fatalf("size = %v", out.Bounds())
	}
}

func TestScaleToFitTallImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 400))
	out := ScaleToFit(src, 120, 100)
	if b := out.Bounds(); b.Dx() != 25 || b.Dy() != 100 {
		t.Fatalf("size = %v, want 25x100", b)
	}
}

func TestDecodeCoverDecodesAndScales(t *testing.T) {
	img, err := DecodeCover(pngBytes(t, 500, 250), 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 100 || b.Dy() != 50 {
		t.Fatalf("size = %v", b)
	}
	if _, err := DecodeCover([]byte("not an image"), 100, 100); err == nil {
		t.Fatal("expected decode error")
	}
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
