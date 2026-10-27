package pair

import (
	"context"
	"strings"
	"sync"

	"github.com/amtypaldos/birdroms-handheld/internal/config"
)

// ResolveServer picks the server URL for first-run pairing: the
// BIRDROMS_SERVER value (env) wins, else the ServerURL already in the config
// file. It returns "" when neither is set; the caller must not invent a host.
func ResolveServer(env string, cfg config.Config) string {
	if v := strings.TrimRight(strings.TrimSpace(env), "/"); v != "" {
		return v
	}
	return strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
}

// Tracker carries Loop's progress from the pairing goroutine to the SDL
// thread, which polls Snapshot every frame without blocking.
type Tracker struct {
	mu   sync.Mutex
	snap Snapshot
}

// Snapshot is a point-in-time copy of a Tracker.
type Snapshot struct {
	HaveSession bool
	Session     Session
	Done        bool
	Config      config.Config
	Err         error
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker { return &Tracker{} }

// SetSession records the current (or restarted) session.
func (t *Tracker) SetSession(s Session) {
	t.mu.Lock()
	t.snap.Session, t.snap.HaveSession = s, true
	t.mu.Unlock()
}

// Finish records the end of the loop.
func (t *Tracker) Finish(cfg config.Config, err error) {
	t.mu.Lock()
	t.snap.Done, t.snap.Config, t.snap.Err = true, cfg, err
	t.mu.Unlock()
}

// Snapshot returns a copy of the current state.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.snap
}

// Run executes Loop and reports into tr. It is meant to run on its own
// goroutine; cancel ctx to abandon pairing.
func Run(ctx context.Context, o Options, tr *Tracker) {
	inner := o.OnSession
	o.OnSession = func(s Session) {
		tr.SetSession(s)
		if inner != nil {
			inner(s)
		}
	}
	cfg, err := Loop(ctx, o)
	tr.Finish(cfg, err)
}
