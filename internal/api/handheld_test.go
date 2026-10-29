package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGetHandheldInfo(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/handheld/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"available":true,"filename":"birdROMs.muxapp","size":42,"sha256":"abc123","published_at":"2026-10-29T10:00:00+00:00"}`))
	})
	got, err := c.GetHandheldInfo()
	if err != nil {
		t.Fatal(err)
	}
	want := HandheldInfo{Available: true, Filename: "birdROMs.muxapp", Size: 42, SHA256: "abc123", PublishedAt: "2026-10-29T10:00:00+00:00"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestGetHandheldInfoNotPublished(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"available":false}`))
	})
	got, err := c.GetHandheldInfo()
	if err != nil {
		t.Fatal(err)
	}
	if got.Available || got.SHA256 != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestDownloadHandheldStreamsAndReportsProgress(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/handheld/download/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte("zipbytes"))
	})
	var buf bytes.Buffer
	var last int64
	if err := c.DownloadHandheld(context.Background(), &buf, func(n int64) { last = n }); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "zipbytes" || last != 8 {
		t.Fatalf("body %q, progress %d", buf.String(), last)
	}
}

func TestDownloadHandheldErrorWritesNothing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not published", http.StatusNotFound)
	})
	var buf bytes.Buffer
	if err := c.DownloadHandheld(context.Background(), &buf, nil); err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("wrote %q on error", buf.String())
	}
}
