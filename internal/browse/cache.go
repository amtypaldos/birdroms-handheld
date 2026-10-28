package browse

import (
	"container/list"
	"image"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

// CoverCacheSize is how many decoded covers are kept at once.
const CoverCacheSize = 32

// Cache is a small LRU keyed by ROM id. It is not safe for concurrent use: the
// SDL thread owns it. When an entry is dropped (evicted, replaced or cleared)
// onEvict is called so the owner can free it, e.g. destroy an SDL texture.
type Cache[V any] struct {
	cap     int
	onEvict func(id int, v V)
	order   *list.List // front = newest
	items   map[int]*list.Element
}

type cacheEntry[V any] struct {
	id int
	v  V
}

// NewCache returns a cache holding at most capacity entries. onEvict may be nil.
func NewCache[V any](capacity int, onEvict func(id int, v V)) *Cache[V] {
	return &Cache[V]{cap: capacity, onEvict: onEvict, order: list.New(), items: map[int]*list.Element{}}
}

// Len is the number of entries held.
func (c *Cache[V]) Len() int { return c.order.Len() }

// Get returns the entry for id and marks it newest.
func (c *Cache[V]) Get(id int) (V, bool) {
	el, ok := c.items[id]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*cacheEntry[V]).v, true
}

// Put stores v for id as the newest entry. Replacing an existing id releases
// the old value; exceeding capacity drops the oldest entry.
func (c *Cache[V]) Put(id int, v V) {
	if el, ok := c.items[id]; ok {
		e := el.Value.(*cacheEntry[V])
		old := e.v
		e.v = v
		c.order.MoveToFront(el)
		c.release(id, old)
		return
	}
	c.items[id] = c.order.PushFront(&cacheEntry[V]{id: id, v: v})
	for c.order.Len() > c.cap {
		el := c.order.Back()
		e := el.Value.(*cacheEntry[V])
		c.order.Remove(el)
		delete(c.items, e.id)
		c.release(e.id, e.v)
	}
}

// Clear drops every entry.
func (c *Cache[V]) Clear() {
	for c.order.Len() > 0 {
		el := c.order.Back()
		e := el.Value.(*cacheEntry[V])
		c.order.Remove(el)
		delete(c.items, e.id)
		c.release(e.id, e.v)
	}
}

func (c *Cache[V]) release(id int, v V) {
	if c.onEvict != nil {
		c.onEvict(id, v)
	}
}

// ---- what is on screen -----------------------------------------------------

// VisibleRows is how many grid rows (of square covers) fit on the 640x480 screen.
const VisibleRows = 2

// ScrollRow returns the first visible grid row after focusing focusRow, moving
// the window as little as possible.
func ScrollRow(focusRow, scroll int) int {
	if focusRow < scroll {
		return focusRow
	}
	if focusRow > scroll+VisibleRows-1 {
		return focusRow - VisibleRows + 1
	}
	return scroll
}

// VisibleRange returns the half-open index range [first, end) of ROMs on
// screen when the grid is scrolled to scrollRow and total ROMs are loaded.
func VisibleRange(scrollRow, total int) (first, end int) {
	first = scrollRow * Cols
	if first > total {
		first = total
	}
	end = first + Cols*VisibleRows
	if end > total {
		end = total
	}
	return first, end
}

// WantCovers returns the ROMs in [first, end) that have a cover to fetch. It
// is the only place that decides a cover may be requested: on screen and a
// non-nil CoverURL.
func WantCovers(roms []api.ROM, first, end int) []api.ROM {
	if end > len(roms) {
		end = len(roms)
	}
	if first < 0 {
		first = 0
	}
	var out []api.ROM
	for i := first; i < end; i++ {
		if roms[i].CoverURL != nil {
			out = append(out, roms[i])
		}
	}
	return out
}

// ---- cover decoding --------------------------------------------------------

// ScaleToFit shrinks src to fit within maxW x maxH keeping its aspect ratio
// (nearest-neighbour; covers are thumbnails). It never upscales.
func ScaleToFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || (w <= maxW && h <= maxH) {
		return src
	}
	nw, nh := maxW, h*maxW/w
	if nh > maxH {
		nw, nh = w*maxH/h, maxH
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*h/nh
		for x := 0; x < nw; x++ {
			dst.Set(x, y, src.At(b.Min.X+x*w/nw, sy))
		}
	}
	return dst
}
