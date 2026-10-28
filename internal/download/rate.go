package download

import (
	"fmt"
	"time"
)

const (
	// rateAlpha is the weight of the newest interval in the smoothed speed.
	rateAlpha = 0.3
	// rateMinInterval is the shortest span worth a speed sample; frames come
	// every ~16 ms, which is too noisy to divide by.
	rateMinInterval = 250 * time.Millisecond
)

// Rate tracks a smoothed transfer speed from (time, bytes-so-far) samples.
// Call Update with each new sample of the current file's byte count.
type Rate struct {
	last      time.Time
	lastBytes int64
	started   bool
	bps       float64
}

// NewRate returns an empty tracker.
func NewRate() *Rate { return &Rate{} }

// Update records that written bytes are on disk at time now. Samples closer
// than rateMinInterval to the last used one are skipped. If written goes
// backwards (the file restarted) the speed is cleared.
func (r *Rate) Update(now time.Time, written int64) {
	if !r.started || written < r.lastBytes {
		r.last, r.lastBytes, r.started, r.bps = now, written, true, 0
		return
	}
	dt := now.Sub(r.last)
	if dt < rateMinInterval {
		return
	}
	inst := float64(written-r.lastBytes) / dt.Seconds()
	if r.bps == 0 {
		r.bps = inst
	} else {
		r.bps = rateAlpha*inst + (1-rateAlpha)*r.bps
	}
	r.last, r.lastBytes = now, written
}

// BytesPerSec is the smoothed speed, 0 until measured.
func (r *Rate) BytesPerSec() float64 { return r.bps }

// ETA is the time to move remaining bytes at the current speed. ok is false
// when the speed is not known yet or nothing remains.
func (r *Rate) ETA(remaining int64) (time.Duration, bool) {
	if r.bps <= 0 || remaining <= 0 {
		return 0, false
	}
	return time.Duration(float64(remaining) / r.bps * float64(time.Second)), true
}

// FormatRate renders a speed as "900 B/s", "1.5 KB/s" or "2.5 MB/s".
func FormatRate(bps float64) string {
	const k = 1024
	switch {
	case bps < k:
		return fmt.Sprintf("%d B/s", int64(bps))
	case bps < k*k:
		return fmt.Sprintf("%.1f KB/s", bps/k)
	default:
		return fmt.Sprintf("%.1f MB/s", bps/(k*k))
	}
}

// FormatDuration renders d as m:ss, or h:mm:ss from an hour up, rounded to
// the nearest second.
func FormatDuration(d time.Duration) string {
	s := int64((d + 500*time.Millisecond) / time.Second)
	if s < 0 {
		s = 0
	}
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Progress is the overall fraction (0..1) of the batch. Done and Failed items
// count as finished. When every size is known it is weighted by bytes and the
// active item contributes what it has written; otherwise it is the share of
// finished items.
func Progress(items []Item) float64 {
	if len(items) == 0 {
		return 0
	}
	var total, have int64
	allSized := true
	finished := 0
	for _, it := range items {
		if it.Total <= 0 {
			allSized = false
		}
		total += it.Total
		switch it.Status {
		case Done, Failed:
			finished++
			have += it.Total
		case Active, Cancelled:
			if it.Written > it.Total {
				have += it.Total
			} else {
				have += it.Written
			}
		}
	}
	if allSized && total > 0 {
		return float64(have) / float64(total)
	}
	return float64(finished) / float64(len(items))
}
