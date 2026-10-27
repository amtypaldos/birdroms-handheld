package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "tok-abc"

// newTestClient starts a mock server running h and returns a client for it.
// The handler also fails the test if the Authorization header is wrong, so
// every endpoint test doubles as an auth-header check.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Device "+testToken; got != want {
			t.Errorf("%s %s: Authorization = %q, want %q", r.Method, r.URL.Path, got, want)
		}
		if r.Method != http.MethodGet {
			t.Errorf("%s %s: method = %s, want GET (read-only client)", r.Method, r.URL.Path, r.Method)
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, testToken, srv.Client()), srv
}

func TestNewTrimsTrailingSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		io.WriteString(w, `[]`)
	}))
	defer srv.Close()
	c := New(srv.URL+"/", testToken, srv.Client())
	if _, err := c.ListPlatforms(); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/roms/platforms/" {
		t.Fatalf("path = %q, want /api/roms/platforms/ (no double slash)", gotPath)
	}
}

func TestListPlatforms(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roms/platforms/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `["GBA","SFC","PSP"]`)
	})
	got, err := c.ListPlatforms()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "GBA,SFC,PSP" {
		t.Fatalf("platforms = %v", got)
	}
}

func TestListROMsParsesPageAndNullFields(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roms/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{
			"count": 50,
			"next": "http://x/api/roms/?fields=device&page=3",
			"previous": "http://x/api/roms/?fields=device",
			"results": [
				{"id": 7, "title": "Metroid Fusion", "platform": "GBA", "extension": "gba", "size_bytes": 8388608, "cover_url": "http://x/c/7.jpg"},
				{"id": 8, "title": "Mystery", "platform": "GBA", "extension": "gba", "size_bytes": null, "cover_url": null}
			]
		}`)
	})
	p, err := c.ListROMs("GBA", 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 50 || len(p.Results) != 2 {
		t.Fatalf("page = %+v", p)
	}
	if p.Next == nil || *p.Next != "http://x/api/roms/?fields=device&page=3" {
		t.Fatalf("Next = %v", p.Next)
	}
	if p.Previous == nil {
		t.Fatal("Previous = nil")
	}
	a, b := p.Results[0], p.Results[1]
	if a.ID != 7 || a.Title != "Metroid Fusion" || a.Platform != "GBA" || a.Extension != "gba" {
		t.Fatalf("rom[0] = %+v", a)
	}
	if a.SizeBytes == nil || *a.SizeBytes != 8388608 || a.CoverURL == nil || *a.CoverURL != "http://x/c/7.jpg" {
		t.Fatalf("rom[0] size/cover = %v / %v", a.SizeBytes, a.CoverURL)
	}
	if b.SizeBytes != nil || b.CoverURL != nil {
		t.Fatalf("rom[1] null fields not nil: %v / %v", b.SizeBytes, b.CoverURL)
	}
}

func TestListROMsLastPageHasNullLinks(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"count":0,"next":null,"previous":null,"results":[]}`)
	})
	p, err := c.ListROMs("", 1)
	if err != nil {
		t.Fatal(err)
	}
	if p.Next != nil || p.Previous != nil || len(p.Results) != 0 {
		t.Fatalf("page = %+v", p)
	}
}

func TestListROMsQueryParams(t *testing.T) {
	var queries []map[string][]string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		io.WriteString(w, `{"count":0,"next":null,"previous":null,"results":[]}`)
	})

	if _, err := c.ListROMs("", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListROMs("Game Boy & Co", 3); err != nil {
		t.Fatal(err)
	}

	q := queries[0]
	if q["fields"][0] != "device" || q["page"][0] != "1" {
		t.Fatalf("query[0] = %v", q)
	}
	if _, ok := q["platform"]; ok {
		t.Fatalf("empty platform must be omitted, got %v", q)
	}
	q = queries[1]
	if q["fields"][0] != "device" || q["page"][0] != "3" || q["platform"][0] != "Game Boy & Co" {
		t.Fatalf("query[1] = %v (platform must round-trip escaped)", q)
	}
}

func TestListPlaylists(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/playlists/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `[
			{"id": 1, "name": "Road trip", "created_by": 7, "created_at": "2026-10-01T12:00:00Z", "rom_count": 3},
			{"id": 2, "name": "Empty", "created_by": 9, "created_at": "2026-10-02T12:00:00Z", "rom_count": 0}
		]`)
	})
	got, err := c.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	p := got[0]
	if p.ID != 1 || p.Name != "Road trip" || p.CreatedBy != 7 || p.CreatedAt != "2026-10-01T12:00:00Z" || p.ROMCount != 3 {
		t.Fatalf("playlist[0] = %+v", p)
	}
}

func TestGetPlaylistKeepsServerROMOrder(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/playlists/42/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		io.WriteString(w, `{
			"id": 42, "name": "Road trip", "created_by": 7, "created_at": "2026-10-01T12:00:00Z", "rom_count": 3,
			"roms": [
				{"id": 30, "title": "C", "platform": "GBA", "extension": "gba", "size_bytes": 1, "cover_url": null},
				{"id": 10, "title": "A", "platform": "SFC", "extension": "sfc", "size_bytes": null, "cover_url": "http://x/a.jpg"},
				{"id": 20, "title": "B", "platform": "GBA", "extension": "gba", "size_bytes": 2, "cover_url": null}
			]
		}`)
	})
	pl, err := c.GetPlaylist(42)
	if err != nil {
		t.Fatal(err)
	}
	if pl.ID != 42 || pl.Name != "Road trip" || pl.ROMCount != 3 {
		t.Fatalf("playlist = %+v", pl)
	}
	var ids []int
	for _, r := range pl.ROMs {
		ids = append(ids, r.ID)
	}
	if len(ids) != 3 || ids[0] != 30 || ids[1] != 10 || ids[2] != 20 {
		t.Fatalf("rom order = %v, want [30 10 20]", ids)
	}
	if pl.ROMs[1].CoverURL == nil || *pl.ROMs[1].CoverURL != "http://x/a.jpg" || pl.ROMs[1].SizeBytes != nil {
		t.Fatalf("rom[1] = %+v", pl.ROMs[1])
	}
}

func TestDownloadROMStreamsBytesAndReportsProgress(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789abcdef"), 10000) // 160000 bytes, several read chunks
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roms/9/download/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write(payload)
	})

	var buf bytes.Buffer
	var calls []int64
	err := c.DownloadROM(9, &buf, func(n int64) { calls = append(calls, n) })
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("downloaded %d bytes, want %d identical", buf.Len(), len(payload))
	}
	if len(calls) == 0 {
		t.Fatal("progress callback never called")
	}
	for i := 1; i < len(calls); i++ {
		if calls[i] <= calls[i-1] {
			t.Fatalf("progress not strictly increasing: %v", calls)
		}
	}
	if last := calls[len(calls)-1]; last != int64(len(payload)) {
		t.Fatalf("final progress = %d, want %d (cumulative)", last, len(payload))
	}
}

func TestDownloadROMNilCallbackAllowed(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "rom-bytes")
	})
	var buf bytes.Buffer
	if err := c.DownloadROM(1, &buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "rom-bytes" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestDownloadROMErrorWritesNothing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found body", http.StatusNotFound)
	})
	var buf bytes.Buffer
	err := c.DownloadROM(1, &buf, nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want one mentioning 404", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("error body leaked into writer: %q", buf.String())
	}
}

func TestUnauthorizedIsSentinelOnEveryCall(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"Invalid token."}`, http.StatusUnauthorized)
	})
	var sink bytes.Buffer
	calls := map[string]func() error{
		"ListPlatforms": func() error { _, err := c.ListPlatforms(); return err },
		"ListROMs":      func() error { _, err := c.ListROMs("GBA", 1); return err },
		"ListPlaylists": func() error { _, err := c.ListPlaylists(); return err },
		"GetPlaylist":   func() error { _, err := c.GetPlaylist(1); return err },
		"DownloadROM":   func() error { return c.DownloadROM(1, &sink, nil) },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: err = %v, want errors.Is ErrUnauthorized", name, err)
		}
	}
	if sink.Len() != 0 {
		t.Fatalf("401 body leaked into download writer: %q", sink.String())
	}
}

func TestOtherStatusIsOrdinaryErrorWithCode(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	_, err := c.ListPlatforms()
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Fatalf("500 reported as ErrUnauthorized: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error %q does not include status code", err)
	}
}

func TestTransportFailureIsNotUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listening any more
	c := New(url, testToken, nil)
	_, err := c.ListPlatforms()
	if err == nil {
		t.Fatal("expected transport error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Fatalf("transport failure reported as ErrUnauthorized: %v", err)
	}
}

func TestMalformedJSONIsError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html>not json</html>`)
	})
	if _, err := c.ListPlaylists(); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGetCoverFetchesAbsoluteURLWithDeviceAuth(t *testing.T) {
	img := []byte("\x89PNG-fake-bytes")
	// The cover lives on a different host than the API base: the client must
	// request the absolute URL as given, not prefix its own base URL.
	cover := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Device "+testToken; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.RequestURI() != "/media/covers/7.jpg?v=2" {
			t.Errorf("request uri = %q", r.URL.RequestURI())
		}
		w.Write(img)
	}))
	defer cover.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API base server must not be hit for an absolute cover url: %s", r.URL)
	}))
	defer api.Close()

	c := New(api.URL, testToken, nil)
	got, err := c.GetCover(cover.URL + "/media/covers/7.jpg?v=2")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, img) {
		t.Fatalf("got %q", got)
	}
}

func TestGetCoverUnauthorizedIsSentinel(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	})
	if _, err := c.GetCover(srv.URL + "/c.jpg"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestGetCoverOtherStatusIsError(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	_, err := c.GetCover(srv.URL + "/c.jpg")
	if err == nil || errors.Is(err, ErrUnauthorized) || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want ordinary error mentioning 404", err)
	}
}

func TestGetCoverRejectsOversizeBody(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), MaxCoverBytes+1))
	})
	if _, err := c.GetCover(srv.URL + "/big.jpg"); err == nil {
		t.Fatal("expected an error for a body over MaxCoverBytes")
	}
}

func TestListPlaylistsCreatedByIsNumber(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id": 1, "name": "x", "created_by": 7, "created_at": "t", "rom_count": 0}]`)
	})
	got, err := c.ListPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].CreatedBy != 7 {
		t.Fatalf("got %+v", got)
	}
}
