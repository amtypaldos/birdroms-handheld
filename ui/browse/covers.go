package browseui

import (
	"errors"
	"image"
	"log"
	"sync"
	"sync/atomic"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/browse"
)

// CoverSource fetches cover bytes; *api.Client satisfies it.
type CoverSource interface {
	GetCover(coverURL string) ([]byte, error)
}

const (
	coverWorkers = 3
	// Decoded covers are shrunk to the cover box on the worker, so the SDL
	// thread only ever uploads small textures.
	coverMaxW, coverMaxH = 136, 136
)

type coverJob struct {
	id  int
	url string
}

type coverResult struct {
	id      int
	img     image.Image // nil on failure or when skipped
	skipped bool        // dropped because the ROM left the screen; not a failure
}

// coverLoader fetches and decodes covers on worker goroutines. The SDL thread
// only enqueues (never blocking), drains results, and sets which ROMs are on
// screen so queued work for ROMs that scrolled away is dropped unfetched.
type coverLoader struct {
	src     CoverSource
	jobs    chan coverJob
	results chan coverResult
	done    chan struct{}

	mu   sync.Mutex
	want map[int]bool

	revoked atomic.Bool
}

func newCoverLoader(src CoverSource) *coverLoader {
	l := &coverLoader{
		src:     src,
		jobs:    make(chan coverJob, 24),
		results: make(chan coverResult, 24),
		done:    make(chan struct{}),
		want:    map[int]bool{},
	}
	for i := 0; i < coverWorkers; i++ {
		go l.worker()
	}
	return l
}

// Close stops the workers. In-flight requests finish (bounded by the HTTP
// client timeout) and their results are dropped.
func (l *coverLoader) Close() { close(l.done) }

// setWanted records the ROM ids currently on screen.
func (l *coverLoader) setWanted(ids map[int]bool) {
	l.mu.Lock()
	l.want = ids
	l.mu.Unlock()
}

func (l *coverLoader) wanted(id int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.want[id]
}

// enqueue queues a fetch without blocking; false means the queue is full and
// the caller should try again on a later frame.
func (l *coverLoader) enqueue(id int, url string) bool {
	select {
	case l.jobs <- coverJob{id: id, url: url}:
		return true
	default:
		return false
	}
}

func (l *coverLoader) worker() {
	for {
		select {
		case <-l.done:
			return
		case j := <-l.jobs:
			l.send(l.fetch(j))
		}
	}
}

func (l *coverLoader) send(r coverResult) {
	select {
	case l.results <- r:
	case <-l.done:
	}
}

func (l *coverLoader) fetch(j coverJob) coverResult {
	if !l.wanted(j.id) {
		return coverResult{id: j.id, skipped: true}
	}
	data, err := l.src.GetCover(j.url)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			l.revoked.Store(true)
		} else {
			log.Printf("cover %d: %v", j.id, err)
		}
		return coverResult{id: j.id}
	}
	img, err := browse.DecodeCover(data, coverMaxW, coverMaxH)
	if err != nil {
		log.Printf("cover %d: %v", j.id, err)
		return coverResult{id: j.id}
	}
	return coverResult{id: j.id, img: img}
}

// drain returns every result that is ready, without blocking.
func (l *coverLoader) drain() []coverResult {
	var out []coverResult
	for {
		select {
		case r := <-l.results:
			out = append(out, r)
		default:
			return out
		}
	}
}
