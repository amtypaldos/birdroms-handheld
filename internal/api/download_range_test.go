package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadROMFromZeroOffsetSendsNoRange(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Range"); got != "" {
			t.Errorf("Range = %q, want none for offset 0", got)
		}
		w.Write([]byte("whole"))
	})
	var buf bytes.Buffer
	if err := c.DownloadROMFrom(context.Background(), 3, 0, &buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "whole" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestDownloadROMFromSendsRangeAndAppendsOn206(t *testing.T) {
	var gotRange string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.Header().Set("Content-Range", "bytes 4-9/10")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("456789"))
	})
	var buf bytes.Buffer
	buf.WriteString("0123") // what is already on disk
	var last int64
	if err := c.DownloadROMFrom(context.Background(), 3, 4, &buf, func(n int64) { last = n }); err != nil {
		t.Fatal(err)
	}
	if gotRange != "bytes=4-" {
		t.Fatalf("Range = %q, want bytes=4-", gotRange)
	}
	if buf.String() != "0123456789" {
		t.Fatalf("got %q", buf.String())
	}
	if last != 10 {
		t.Fatalf("final progress = %d, want 10 (includes the 4 existing bytes)", last)
	}
}

func TestDownloadROMFromRestartsFileOn200(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fresh-body")) // server ignored the Range header
	})
	f, err := os.Create(filepath.Join(t.TempDir(), "x.part"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.WriteString("stale")
	f.Seek(0, 2)

	var last int64
	if err := c.DownloadROMFrom(context.Background(), 3, 5, f, func(n int64) { last = n }); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(f.Name())
	if string(got) != "fresh-body" {
		t.Fatalf("file = %q, want the 200 body only", got)
	}
	if last != int64(len("fresh-body")) {
		t.Fatalf("final progress = %d, want %d", last, len("fresh-body"))
	}
}

func TestDownloadROMFromWriterThatCannotRestartIsError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fresh-body"))
	})
	var buf bytes.Buffer
	buf.WriteString("stale")
	err := c.DownloadROMFrom(context.Background(), 3, 5, &buf, nil)
	if err == nil {
		t.Fatal("want an error: a 200 to a ranged request cannot be appended")
	}
	if buf.String() != "stale" {
		t.Fatalf("writer was modified: %q", buf.String())
	}
}

func TestDownloadROMFromUnauthorizedIsSentinel(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	err := c.DownloadROMFrom(context.Background(), 3, 4, &bytes.Buffer{}, nil)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestDownloadROMFromRangeNotSatisfiableIsSentinel(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	})
	err := c.DownloadROMFrom(context.Background(), 3, 99, &bytes.Buffer{}, nil)
	if !errors.Is(err, ErrRangeNotSatisfiable) {
		t.Fatalf("err = %v, want ErrRangeNotSatisfiable", err)
	}
	if err != nil && !strings.Contains(err.Error(), "416") {
		t.Fatalf("err = %v, want one mentioning 416", err)
	}
}

func TestDownloadROMFromMisalignedContentRangeIsErrorAndWritesNothing(t *testing.T) {
	for name, hdr := range map[string]string{
		"wrong start": "bytes 0-5/10",
		"missing":     "",
		"garbage":     "items 4-9/10",
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if hdr != "" {
					w.Header().Set("Content-Range", hdr)
				}
				w.WriteHeader(http.StatusPartialContent)
				w.Write([]byte("456789"))
			})
			var buf bytes.Buffer
			buf.WriteString("0123")
			if err := c.DownloadROMFrom(context.Background(), 3, 4, &buf, nil); err == nil {
				t.Fatal("want an error for a 206 that does not start at the offset we sent")
			}
			if buf.String() != "0123" {
				t.Fatalf("writer = %q, want it untouched", buf.String())
			}
		})
	}
}

type failAfterWriter struct{ n int }

func (f *failAfterWriter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("disk full")
	}
	if len(p) > f.n {
		p = p[:f.n]
	}
	f.n -= len(p)
	return len(p), nil
}

func TestDownloadROMFromWriterErrorMidCopyIsReturned(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0123456789"))
	})
	if err := c.DownloadROMFrom(context.Background(), 3, 0, &failAfterWriter{n: 4}, nil); err == nil {
		t.Fatal("want the writer's error")
	}
}

func TestDownloadROMFromCancelledBeforeResponseReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		cancel()
		select { // never answers; the timeout only keeps a failing run from hanging
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	})
	var buf bytes.Buffer
	err := c.DownloadROMFrom(ctx, 3, 0, &buf, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("writer got %q", buf.String())
	}
}
