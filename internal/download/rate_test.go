package download

import (
	"math"
	"testing"
	"time"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

var t0 = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func TestRateFirstIntervalSetsTheSpeed(t *testing.T) {
	r := NewRate()
	r.Update(t0, 0)
	r.Update(t0.Add(time.Second), 1<<20)
	if got := r.BytesPerSec(); math.Abs(got-float64(1<<20)) > 1 {
		t.Fatalf("speed = %v, want 1 MiB/s", got)
	}
}

func TestRateSmoothsASpike(t *testing.T) {
	r := NewRate()
	r.Update(t0, 0)
	r.Update(t0.Add(time.Second), 1000)
	r.Update(t0.Add(2*time.Second), 11000) // one 10x burst
	got := r.BytesPerSec()
	if got <= 1000 || got >= 10000 {
		t.Fatalf("speed = %v, want a smoothed value between the old and the burst rate", got)
	}
}

func TestRateIgnoresVeryShortIntervals(t *testing.T) {
	r := NewRate()
	r.Update(t0, 0)
	r.Update(t0.Add(time.Second), 1000)
	before := r.BytesPerSec()
	r.Update(t0.Add(time.Second+10*time.Millisecond), 1000000) // 10 ms later: too noisy to use
	if r.BytesPerSec() != before {
		t.Fatalf("a 10 ms sample changed the speed: %v -> %v", before, r.BytesPerSec())
	}
}

func TestRateResetsWhenBytesGoBackwards(t *testing.T) {
	r := NewRate()
	r.Update(t0, 0)
	r.Update(t0.Add(time.Second), 5000)
	r.Update(t0.Add(2*time.Second), 100) // the file restarted
	if got := r.BytesPerSec(); got != 0 {
		t.Fatalf("speed after a restart = %v, want 0 until measured again", got)
	}
}

func TestETA(t *testing.T) {
	r := NewRate()
	if _, ok := r.ETA(1000); ok {
		t.Fatal("ETA known before any speed")
	}
	r.Update(t0, 0)
	r.Update(t0.Add(time.Second), 1000)
	d, ok := r.ETA(5000)
	if !ok || d != 5*time.Second {
		t.Fatalf("ETA = %v, %v, want 5s", d, ok)
	}
	if _, ok := r.ETA(0); ok {
		t.Fatal("ETA for nothing remaining should not be shown")
	}
}

func TestFormatRate(t *testing.T) {
	cases := map[float64]string{0: "0 B/s", 900: "900 B/s", 1536: "1.5 KB/s", 2.5 * 1024 * 1024: "2.5 MB/s"}
	for in, want := range cases {
		if got := FormatRate(in); got != want {
			t.Errorf("FormatRate(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                               "0:00",
		18 * time.Second:                "0:18",
		65 * time.Second:                "1:05",
		(3600 + 5*60 + 9) * time.Second: "1:05:09",
		1500 * time.Millisecond:         "0:02", // rounded to the nearest second
	}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", in, got, want)
		}
	}
}

func sz(n int64) *int64 { return &n }

func TestOverallProgressWeighsByBytesWhenAllSizesKnown(t *testing.T) {
	items := []Item{
		{ROM: api.ROM{SizeBytes: sz(100)}, Total: 100, Status: Done, Written: 100},
		{ROM: api.ROM{SizeBytes: sz(300)}, Total: 300, Status: Active, Written: 150},
		{ROM: api.ROM{SizeBytes: sz(100)}, Total: 100, Status: Pending},
	}
	// (100 + 150 + 0) / 500
	if got := Progress(items); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("Progress = %v, want 0.5", got)
	}
}

func TestOverallProgressFallsBackToCountWhenASizeIsUnknown(t *testing.T) {
	items := []Item{
		{Status: Done},
		{Status: Done},
		{Status: Active, Written: 10},
		{Status: Pending},
	}
	if got := Progress(items); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("Progress = %v, want 0.5 (2 of 4 items finished)", got)
	}
}

func TestOverallProgressCountsFailedAsFinishedAndHandlesEmpty(t *testing.T) {
	if got := Progress(nil); got != 0 {
		t.Fatalf("Progress(nil) = %v", got)
	}
	items := []Item{{Status: Failed}, {Status: Pending}}
	if got := Progress(items); got != 0.5 {
		t.Fatalf("Progress = %v, want 0.5", got)
	}
}
