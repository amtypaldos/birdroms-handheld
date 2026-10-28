package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/platforms"
)

// Fetcher downloads a ROM into w starting at offset. *api.Client implements it.
type Fetcher interface {
	DownloadROMFrom(ctx context.Context, id int, offset int64, w io.Writer, progress func(written int64)) error
}

// Status is where an item is in the queue.
type Status int

const (
	Pending   Status = iota // not started
	Active                  // downloading now
	Done                    // final file is in place (written now or already there)
	Failed                  // error; see Item.Err
	Cancelled               // stopped mid-download by the context; the .part stays
)

func (s Status) String() string {
	switch s {
	case Active:
		return "downloading"
	case Done:
		return "done"
	case Failed:
		return "failed"
	case Cancelled:
		return "stopped"
	default:
		return "pending"
	}
}

// Item is one queued ROM. Dir is the muOS folder name under Roms; Known is
// false when the platform had no folder and the ROM goes to the visible
// fallback folder (platforms.Unsorted).
type Item struct {
	ROM     api.ROM
	Status  Status
	Written int64 // bytes on disk so far, including a resumed .part
	Total   int64 // ROM size if the server reported it, else 0
	Dir     string
	Known   bool
	Skipped bool // the ROM was already on the card; nothing was downloaded
	// Existing is the card's file name when Skipped matched a differently
	// named file (e.g. "Zelda (USA).gba" for the ROM titled "Zelda").
	Existing string
	Err      string
}

// Queue downloads its items one at a time into <romMount>/Roms/<dir>.
type Queue struct {
	// Refresh is called with the system directory after each ROM is renamed
	// into place. The default is a no-op; muOS game-list refresh is not
	// verified, and this is where a real command would go. A Refresh error is
	// logged and does not fail the download.
	Refresh func(dir string) error

	// Art, when set, is called after a ROM was downloaded and renamed into
	// place, with the muOS folder name and the file name on the card (the name
	// muOS keys box art to). It runs in its own goroutine so it never delays
	// the next download; an error is only logged. Run waits up to ArtWait for
	// pending calls before it returns. ROMs skipped as already there get no call.
	Art func(rom api.ROM, dir, fileName string) error
	// ArtWait bounds how long Run waits for pending Art calls (default 10s).
	ArtWait time.Duration

	art   sync.WaitGroup
	f     Fetcher
	mount string
	// libs are other ROM storages (e.g. the SD card when downloads go to the
	// internal one). They are only read, to spot a ROM that is already there.
	libs []string

	mu       sync.Mutex
	items    []Item
	finished bool
}

// New returns a queue for roms. romMount is the ROM card mount (from
// BIRDROMS_ROM_MOUNT); with an empty mount every item fails without
// downloading anything.
func New(f Fetcher, romMount string, roms []api.ROM) *Queue {
	q := &Queue{
		Refresh: func(string) error { return nil },
		f:       f,
		mount:   romMount,
		items:   make([]Item, len(roms)),
	}
	for i, r := range roms {
		dir, known := platforms.Folder(r.Platform)
		q.items[i] = Item{ROM: r, Dir: dir, Known: known}
		if r.SizeBytes != nil {
			q.items[i].Total = *r.SizeBytes
		}
	}
	return q
}

// SetLibraries tells the queue about other ROM storages to check for a ROM
// that is already there. Empty entries and the download target are ignored.
func (q *Queue) SetLibraries(roots ...string) {
	q.libs = q.libs[:0]
	for _, r := range roots {
		if r != "" && r != q.mount {
			q.libs = append(q.libs, r)
		}
	}
}

// Snapshot returns a copy of the items for drawing.
func (q *Queue) Snapshot() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]Item(nil), q.items...)
}

// Finished reports whether Run has returned.
func (q *Queue) Finished() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.finished
}

// Run downloads the items in order, one at a time, and returns when all are
// processed or ctx is cancelled. A failed item does not stop the batch. On
// cancel, finished files and the in-flight .part stay and later items stay
// Pending. Call it once.
func (q *Queue) Run(ctx context.Context) {
	defer func() {
		q.waitArt()
		q.mu.Lock()
		q.finished = true
		q.mu.Unlock()
	}()
	for i := range q.items {
		if ctx.Err() != nil {
			return
		}
		q.runOne(ctx, i)
	}
}

func (q *Queue) startArt(rom api.ROM, dir, file string) {
	if q.Art == nil {
		return
	}
	q.art.Add(1)
	go func() {
		defer q.art.Done()
		if err := q.Art(rom, dir, file); err != nil {
			log.Printf("download: box art for %s/%s: %v", dir, file, err)
		}
	}()
}

// waitArt gives pending Art calls a bounded time to finish so the process
// does not exit mid-write; a stuck one is abandoned (its temp file is dot-named).
func (q *Queue) waitArt() {
	d := q.ArtWait
	if d <= 0 {
		d = 10 * time.Second
	}
	done := make(chan struct{})
	go func() { q.art.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		log.Printf("download: gave up waiting for box art")
	}
}

func (q *Queue) update(i int, fn func(*Item)) {
	q.mu.Lock()
	fn(&q.items[i])
	q.mu.Unlock()
}

func (q *Queue) fail(i int, msg string) {
	q.update(i, func(it *Item) { it.Status, it.Err = Failed, msg })
}

func (q *Queue) runOne(ctx context.Context, i int) {
	it := q.Snapshot()[i]
	if q.mount == "" {
		q.fail(i, "BIRDROMS_ROM_MOUNT is not set")
		return
	}
	// it.Dir is always a name from the folder map (or the fallback), never
	// the raw platform string.
	dir := filepath.Join(q.mount, "Roms", it.Dir)
	// Another storage may already hold it (muOS merges them in its game list).
	// Checked before anything is created so a skip leaves no empty folders.
	for _, lib := range q.libs {
		if name, ok := findExisting(filepath.Join(lib, "Roms", it.Dir), it.ROM); ok {
			size := *it.ROM.SizeBytes
			q.update(i, func(it *Item) { it.Status, it.Skipped, it.Existing, it.Written = Done, true, name, size })
			return
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		q.fail(i, err.Error())
		return
	}
	// The card may already hold this ROM under another name (a library copied
	// over from elsewhere): same extension, same title once tags are ignored,
	// same size. Never download that again.
	if name, ok := findExisting(dir, it.ROM); ok {
		size := *it.ROM.SizeBytes
		q.update(i, func(it *Item) { it.Status, it.Skipped, it.Existing, it.Written = Done, true, name, size })
		return
	}
	// A file already at the clean name may be a different ROM with the same
	// title, so it is never overwritten or taken as "already downloaded".
	// Fall back to <title>-<id><ext>; if that exists this ROM was saved before.
	final := filepath.Join(dir, Filename(it.ROM))
	if _, err := os.Stat(final); err == nil {
		final = filepath.Join(dir, idFilename(it.ROM))
		if st, err := os.Stat(final); err == nil {
			q.update(i, func(it *Item) { it.Status, it.Skipped, it.Written = Done, true, st.Size() })
			return
		}
	}

	q.update(i, func(it *Item) { it.Status = Active })
	err := q.fetchToPart(ctx, i, final)
	switch {
	case err == nil:
		q.update(i, func(it *Item) { it.Status = Done })
		if rerr := q.Refresh(dir); rerr != nil {
			log.Printf("download: refresh %s: %v", dir, rerr)
		}
		q.startArt(it.ROM, it.Dir, filepath.Base(final))
	case ctx.Err() != nil:
		q.update(i, func(it *Item) { it.Status = Cancelled })
	default:
		q.fail(i, err.Error())
	}
}

// fetchToPart downloads item i into final+".part" (resuming from its current
// size), syncs it and renames it to final. On error the final name is not
// created and the .part is left in place.
func (q *Queue) fetchToPart(ctx context.Context, i int, final string) error {
	part := final + ".part"
	f, err := os.OpenFile(part, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	err = q.fill(ctx, i, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(part, final)
}

// fill downloads into the open part file f, continuing from its current size,
// and syncs it.
func (q *Queue) fill(ctx context.Context, i int, f *os.File) error {
	st, err := f.Stat()
	if err != nil {
		return err
	}
	offset := st.Size()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	err = q.fetch(ctx, i, f, offset)
	if errors.Is(err, api.ErrRangeNotSatisfiable) && offset > 0 {
		// The .part is already as long as (or longer than) the ROM, e.g. a
		// crash after the last byte. Start over once.
		if err = restart(f); err == nil {
			err = q.fetch(ctx, i, f, 0)
		}
	}
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	st, err = f.Stat()
	if err != nil {
		return err
	}
	q.update(i, func(it *Item) { it.Written = st.Size() })
	// A body cut short without an error (no Content-Length, closed early) must
	// not be renamed into place; the .part stays for a resume.
	if want := q.items[i].ROM.SizeBytes; want != nil && *want > 0 && st.Size() != *want {
		return fmt.Errorf("download incomplete: %d of %d bytes", st.Size(), *want)
	}
	return nil
}

// fetch reads the ROM id without the lock: ROM is set in New and never changes.
func (q *Queue) fetch(ctx context.Context, i int, f *os.File, offset int64) error {
	q.update(i, func(it *Item) { it.Written = offset })
	cw := &ctxFile{ctx: ctx, f: f}
	return q.f.DownloadROMFrom(ctx, q.items[i].ROM.ID, offset, cw, func(written int64) {
		q.update(i, func(it *Item) { it.Written = written })
	})
}

func restart(f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := f.Seek(0, io.SeekStart)
	return err
}

// ctxFile is the writer handed to the Fetcher: writes stop with the context's
// error once it is cancelled (this is how B interrupts a body mid-download),
// and it keeps Seek/Truncate so a 200 answer to a ranged request can restart
// the file.
type ctxFile struct {
	ctx context.Context
	f   *os.File
}

func (c *ctxFile) Write(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, fmt.Errorf("download cancelled: %w", err)
	}
	return c.f.Write(p)
}

func (c *ctxFile) Seek(off int64, whence int) (int64, error) { return c.f.Seek(off, whence) }
func (c *ctxFile) Truncate(size int64) error                 { return c.f.Truncate(size) }
