package pair

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amtypaldos/birdroms-handheld/internal/config"
)

func TestResolveServer(t *testing.T) {
	cases := []struct {
		name string
		env  string
		cfg  config.Config
		want string
	}{
		{"nothing", "", config.Config{}, ""},
		{"env only", "https://a.example", config.Config{}, "https://a.example"},
		{"env trimmed", "  https://a.example/ \n", config.Config{}, "https://a.example"},
		{"file url, no token", "", config.Config{ServerURL: "https://b.example/"}, "https://b.example"},
		{"env beats file", "https://a.example", config.Config{ServerURL: "https://b.example"}, "https://a.example"},
		{"whitespace env ignored", "   ", config.Config{ServerURL: "https://b.example"}, "https://b.example"},
	}
	for _, c := range cases {
		if got := ResolveServer(c.env, c.cfg); got != c.want {
			t.Errorf("%s: ResolveServer = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTrackerSnapshotFollowsLoop(t *testing.T) {
	tr := NewTracker()
	if s := tr.Snapshot(); s.HaveSession || s.Done {
		t.Fatalf("fresh snapshot = %+v", s)
	}
	tr.SetSession(Session{Code: "AB-12", QRURL: "https://x/q"})
	s := tr.Snapshot()
	if !s.HaveSession || s.Session.Code != "AB-12" || s.Done {
		t.Fatalf("after SetSession = %+v", s)
	}
	// A restart replaces the session.
	tr.SetSession(Session{Code: "CD-34", QRURL: "https://x/q2"})
	if got := tr.Snapshot().Session.Code; got != "CD-34" {
		t.Fatalf("session code = %q", got)
	}
	tr.Finish(config.Config{ServerURL: "u", DeviceToken: "t"}, nil)
	s = tr.Snapshot()
	if !s.Done || s.Err != nil || !s.Config.Paired() {
		t.Fatalf("after Finish = %+v", s)
	}
}

func TestTrackerFinishError(t *testing.T) {
	tr := NewTracker()
	tr.Finish(config.Config{}, ErrExpired)
	s := tr.Snapshot()
	if !s.Done || !errors.Is(s.Err, ErrExpired) {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestRunWiresTrackerAndSaves(t *testing.T) {
	f := newFakeServer(t, []string{"RUN-1"}, map[string][]pollReply{"RUN-1": {pending(), approved()}})
	opts, path := f.opts(t, &recorder{})
	tr := NewTracker()
	done := make(chan struct{})
	go func() { Run(context.Background(), opts, tr); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not finish")
	}
	s := tr.Snapshot()
	if !s.Done || s.Err != nil || s.Config.DeviceToken != fakeToken || s.Session.Code != "RUN-1" {
		t.Fatalf("snapshot = %+v", s)
	}
	if got, err := config.Load(path); err != nil || got.DeviceToken != fakeToken {
		t.Fatalf("saved = %+v err = %v", got, err)
	}
}
