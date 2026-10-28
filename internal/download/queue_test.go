package download

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

// fakeFetcher runs fn for each download and records the calls.
type fakeFetcher struct {
	fn    func(id int, offset int64, w io.Writer, progress func(int64)) error
	calls []int
}

func (f *fakeFetcher) DownloadROMFrom(_ context.Context, id int, offset int64, w io.Writer, progress func(int64)) error {
	f.calls = append(f.calls, id)
	return f.fn(id, offset, w, progress)
}

// body returns a fetcher fn that writes content.
func body(content string) func(int, int64, io.Writer, func(int64)) error {
	return func(_ int, _ int64, w io.Writer, progress func(int64)) error {
		n, err := io.WriteString(w, content)
		if progress != nil {
			progress(int64(n))
		}
		return err
	}
}

func gba(id int, title string) api.ROM {
	return api.ROM{ID: id, Title: title, Platform: "Game Boy Advance", Extension: ".gba"}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRunWritesROMIntoMappedFolder(t *testing.T) {
	mount := t.TempDir()
	q := New(&fakeFetcher{fn: body("rom-bytes")}, mount, []api.ROM{gba(1, "Metroid")})
	q.Run(context.Background())

	final := filepath.Join(mount, "Roms", "GBA", "Metroid.gba")
	if got := readFile(t, final); got != "rom-bytes" {
		t.Fatalf("final = %q", got)
	}
	if exists(final + ".part") {
		t.Fatal(".part left behind after success")
	}
	it := q.Snapshot()[0]
	if it.Status != Done || it.Written != int64(len("rom-bytes")) || !it.Known || it.Dir != "GBA" {
		t.Fatalf("item = %+v", it)
	}
}

func TestInterruptedWriteLeavesNoFinalFile(t *testing.T) {
	mount := t.TempDir()
	fetch := &fakeFetcher{fn: func(_ int, _ int64, w io.Writer, _ func(int64)) error {
		io.WriteString(w, "half")
		return errors.New("connection reset")
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "Metroid")})
	q.Run(context.Background())

	dir := filepath.Join(mount, "Roms", "GBA")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".part") {
			t.Fatalf("non-.part file %q in the games folder after a failed download", e.Name())
		}
	}
	it := q.Snapshot()[0]
	if it.Status != Failed || !strings.Contains(it.Err, "connection reset") {
		t.Fatalf("item = %+v", it)
	}
}

func TestResumeAppendsTo206AndSendsRange(t *testing.T) {
	var gotRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		if r.Header.Get("Authorization") != "Device tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Range", "bytes 4-9/10")
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "456789")
	}))
	defer srv.Close()

	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("0123"), 0o644)

	q := New(api.New(srv.URL, "tok", nil), mount, []api.ROM{gba(1, "Metroid")})
	q.Run(context.Background())

	if gotRange != "bytes=4-" {
		t.Fatalf("Range = %q, want bytes=4-", gotRange)
	}
	final := filepath.Join(dir, "Metroid.gba")
	if got := readFile(t, final); got != "0123456789" {
		t.Fatalf("final = %q", got)
	}
	if exists(final + ".part") {
		t.Fatal(".part left behind")
	}
	if it := q.Snapshot()[0]; it.Status != Done || it.Written != 10 {
		t.Fatalf("item = %+v, want Done with cumulative Written 10", it)
	}
}

func TestRangedRequestAnswered200ReplacesPartFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=5-" {
			t.Errorf("Range = %q", r.Header.Get("Range"))
		}
		io.WriteString(w, "complete-body") // ignores the range
	}))
	defer srv.Close()

	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("stale"), 0o644)

	q := New(api.New(srv.URL, "tok", nil), mount, []api.ROM{gba(1, "Metroid")})
	q.Run(context.Background())

	if got := readFile(t, filepath.Join(dir, "Metroid.gba")); got != "complete-body" {
		t.Fatalf("final = %q, want the 200 body only (not stale+body)", got)
	}
}

func TestPartAlreadyFullLengthRestartsFromZero(t *testing.T) {
	// A crash between the last byte and the rename leaves a complete .part;
	// the server answers the ranged request with 416. Start over, once.
	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("complete"), 0o644)

	var offsets []int64
	fetch := &fakeFetcher{fn: func(id int, offset int64, w io.Writer, p func(int64)) error {
		offsets = append(offsets, offset)
		if offset > 0 {
			return errors.Join(errors.New("status 416"), api.ErrRangeNotSatisfiable)
		}
		return body("complete")(id, 0, w, p)
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "Metroid")})
	q.Run(context.Background())

	if len(offsets) != 2 || offsets[0] != 8 || offsets[1] != 0 {
		t.Fatalf("offsets = %v, want [8 0]", offsets)
	}
	if got := readFile(t, filepath.Join(dir, "Metroid.gba")); got != "complete" {
		t.Fatalf("final = %q", got)
	}
}

func TestHostilePlatformAndTitleStayUnderUnsorted(t *testing.T) {
	root := t.TempDir()
	mount := filepath.Join(root, "card")
	rom := api.ROM{ID: 9, Title: "../x", Platform: "../../etc", Extension: ".bin"}
	q := New(&fakeFetcher{fn: body("evil")}, mount, []api.ROM{rom})
	q.Run(context.Background())

	unsorted := filepath.Join(mount, "Roms", "_UNSORTED") + string(filepath.Separator)
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 1 || !strings.HasPrefix(files[0], unsorted) {
		t.Fatalf("files = %v, want exactly one under %s", files, unsorted)
	}
	if it := q.Snapshot()[0]; it.Known || it.Dir != "_UNSORTED" || it.Status != Done {
		t.Fatalf("item = %+v, want Done, flagged unknown, in _UNSORTED", it)
	}
}

func TestOneFailureDoesNotStopTheNextItem(t *testing.T) {
	mount := t.TempDir()
	fetch := &fakeFetcher{fn: func(id int, off int64, w io.Writer, p func(int64)) error {
		if id == 1 {
			return errors.New("boom")
		}
		return body("ok")(id, off, w, p)
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "First"), gba(2, "Second")})
	q.Run(context.Background())

	items := q.Snapshot()
	if items[0].Status != Failed || items[1].Status != Done {
		t.Fatalf("statuses = %v, %v; want Failed, Done", items[0].Status, items[1].Status)
	}
	if got := readFile(t, filepath.Join(mount, "Roms", "GBA", "Second.gba")); got != "ok" {
		t.Fatalf("second = %q", got)
	}
}

func TestRunsOneROMAtATime(t *testing.T) {
	mount := t.TempDir()
	active, maxActive := 0, 0
	fetch := &fakeFetcher{fn: func(id int, off int64, w io.Writer, p func(int64)) error {
		active++
		if active > maxActive {
			maxActive = active
		}
		defer func() { active-- }()
		return body("x")(id, off, w, p)
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "A"), gba(2, "B"), gba(3, "C")})
	q.Run(context.Background())
	if maxActive != 1 || len(fetch.calls) != 3 {
		t.Fatalf("maxActive = %d, calls = %v", maxActive, fetch.calls)
	}
}

func TestRefreshCalledWithSystemDirOnlyAfterRename(t *testing.T) {
	mount := t.TempDir()
	fetch := &fakeFetcher{fn: func(id int, off int64, w io.Writer, p func(int64)) error {
		if id == 1 {
			io.WriteString(w, "half")
			return errors.New("boom")
		}
		return body("ok")(id, off, w, p)
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "Bad"), gba(2, "Good")})
	var dirs []string
	finalExisted := false
	q.Refresh = func(dir string) error {
		dirs = append(dirs, dir)
		finalExisted = exists(filepath.Join(dir, "Good.gba"))
		return nil
	}
	q.Run(context.Background())

	want := filepath.Join(mount, "Roms", "GBA")
	if len(dirs) != 1 || dirs[0] != want {
		t.Fatalf("Refresh calls = %v, want exactly [%s] (not for the failed item)", dirs, want)
	}
	if !finalExisted {
		t.Fatal("Refresh ran before the final file existed")
	}
}

func TestRefreshErrorDoesNotFailTheDownload(t *testing.T) {
	mount := t.TempDir()
	q := New(&fakeFetcher{fn: body("ok")}, mount, []api.ROM{gba(1, "A")})
	q.Refresh = func(string) error { return errors.New("refresh broke") }
	q.Run(context.Background())
	if it := q.Snapshot()[0]; it.Status != Done {
		t.Fatalf("status = %v, want Done", it.Status)
	}
}

func TestExistingFinalFileIsSkippedNotOverwritten(t *testing.T) {
	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba"), []byte("mine"), 0o644)
	os.WriteFile(filepath.Join(dir, "Metroid-1.gba"), []byte("mine-1"), 0o644) // this ROM was saved before

	fetch := &fakeFetcher{fn: body("theirs")}
	q := New(fetch, mount, []api.ROM{gba(1, "Metroid")})
	refreshed := false
	q.Refresh = func(string) error { refreshed = true; return nil }
	q.Run(context.Background())

	if len(fetch.calls) != 0 {
		t.Fatalf("fetcher called for an existing file: %v", fetch.calls)
	}
	if got := readFile(t, filepath.Join(dir, "Metroid.gba")); got != "mine" {
		t.Fatalf("existing file overwritten: %q", got)
	}
	if it := q.Snapshot()[0]; it.Status != Done || !it.Skipped {
		t.Fatalf("item = %+v, want Done and Skipped", it)
	}
	if refreshed {
		t.Fatal("Refresh called although nothing was written")
	}
}

func TestCancelKeepsFinishedFilesAndPartAndLeavesLaterPending(t *testing.T) {
	mount := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fetch := &fakeFetcher{fn: func(id int, off int64, w io.Writer, p func(int64)) error {
		if id == 2 {
			io.WriteString(w, "part-of-second")
			cancel() // the user pressed B mid-download
			if _, err := w.Write([]byte("more")); err == nil {
				t.Error("write after cancel succeeded; the body must stop")
			}
			return ctx.Err()
		}
		return body("ok")(id, off, w, p)
	}}
	q := New(fetch, mount, []api.ROM{gba(1, "First"), gba(2, "Second"), gba(3, "Third")})
	q.Run(ctx)

	dir := filepath.Join(mount, "Roms", "GBA")
	if got := readFile(t, filepath.Join(dir, "First.gba")); got != "ok" {
		t.Fatalf("finished file lost: %q", got)
	}
	if exists(filepath.Join(dir, "Second.gba")) {
		t.Fatal("cancelled item has a final file")
	}
	if !exists(filepath.Join(dir, "Second.gba.part")) {
		t.Fatal("in-flight .part was removed")
	}
	if len(fetch.calls) != 2 {
		t.Fatalf("calls = %v; the third item must not start", fetch.calls)
	}
	items := q.Snapshot()
	if items[0].Status != Done || items[1].Status != Cancelled || items[2].Status != Pending {
		t.Fatalf("statuses = %v %v %v", items[0].Status, items[1].Status, items[2].Status)
	}
}

func TestMissingROMMountFailsEveryItemWithoutDownloading(t *testing.T) {
	fetch := &fakeFetcher{fn: body("x")}
	q := New(fetch, "", []api.ROM{gba(1, "A"), gba(2, "B")})
	q.Run(context.Background())
	if len(fetch.calls) != 0 {
		t.Fatalf("downloaded with no ROM mount: %v", fetch.calls)
	}
	for _, it := range q.Snapshot() {
		if it.Status != Failed || !strings.Contains(it.Err, "BIRDROMS_ROM_MOUNT") {
			t.Fatalf("item = %+v", it)
		}
	}
}

func TestProgressAndFinishedReporting(t *testing.T) {
	mount := t.TempDir()
	size := int64(10)
	rom := gba(1, "A")
	rom.SizeBytes = &size
	q := New(&fakeFetcher{fn: body("0123456789")}, mount, []api.ROM{rom})
	if q.Finished() {
		t.Fatal("Finished before Run")
	}
	if it := q.Snapshot()[0]; it.Status != Pending || it.Total != 10 {
		t.Fatalf("before run: %+v", it)
	}
	q.Run(context.Background())
	if !q.Finished() {
		t.Fatal("not Finished after Run")
	}
}

// sizedROM is gba(id, title) with a known size.
func sizedROM(id int, title string, size int64) api.ROM {
	r := gba(id, title)
	r.SizeBytes = &size
	return r
}

func partDir(mount string) string { return filepath.Join(mount, "Roms", "GBA") }

func TestShortBodyWithoutContentLengthIsNotRenamed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "abc")
		w.(http.Flusher).Flush() // chunked: the client sees no Content-Length
	}))
	defer srv.Close()

	mount := t.TempDir()
	q := New(api.New(srv.URL, "tok", nil), mount, []api.ROM{sizedROM(1, "Metroid", 10)})
	q.Run(context.Background())

	final := filepath.Join(partDir(mount), "Metroid.gba")
	if exists(final) {
		t.Fatal("a short download was renamed into place")
	}
	if !exists(final + ".part") {
		t.Fatal(".part was removed; it must stay for a resume")
	}
	if it := q.Snapshot()[0]; it.Status != Failed {
		t.Fatalf("item = %+v, want Failed", it)
	}
}

func TestWrongContentRangeStartIsNotRenamedOrAppended(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 0-5/10") // we asked for 4-
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "456789")
	}))
	defer srv.Close()

	mount := t.TempDir()
	dir := partDir(mount)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("0123"), 0o644)

	q := New(api.New(srv.URL, "tok", nil), mount, []api.ROM{sizedROM(1, "Metroid", 10)})
	q.Run(context.Background())

	if exists(filepath.Join(dir, "Metroid.gba")) {
		t.Fatal("a misaligned 206 produced a final file")
	}
	if got := readFile(t, filepath.Join(dir, "Metroid.gba.part")); got != "0123" {
		t.Fatalf(".part = %q, want it untouched", got)
	}
}

func TestFailingWriterMidCopyDoesNotCreateFinalFile(t *testing.T) {
	fetch := &fakeFetcher{fn: func(_ int, _ int64, w io.Writer, _ func(int64)) error {
		io.WriteString(w, "half")
		return errors.New("disk error")
	}}
	mount := t.TempDir()
	q := New(fetch, mount, []api.ROM{sizedROM(1, "Metroid", 10)})
	q.Run(context.Background())
	if exists(filepath.Join(partDir(mount), "Metroid.gba")) {
		t.Fatal("final file created after a failed copy")
	}
}

func TestFullBodyMatchingSizeStillRenamesIncludingResume(t *testing.T) {
	// fresh
	mount := t.TempDir()
	q := New(&fakeFetcher{fn: body("0123456789")}, mount, []api.ROM{sizedROM(1, "Metroid", 10)})
	q.Run(context.Background())
	if got := readFile(t, filepath.Join(partDir(mount), "Metroid.gba")); got != "0123456789" {
		t.Fatalf("fresh final = %q", got)
	}

	// resume over HTTP
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 4-9/10")
		w.WriteHeader(http.StatusPartialContent)
		io.WriteString(w, "456789")
	}))
	defer srv.Close()
	mount = t.TempDir()
	dir := partDir(mount)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("0123"), 0o644)
	q = New(api.New(srv.URL, "tok", nil), mount, []api.ROM{sizedROM(1, "Metroid", 10)})
	q.Run(context.Background())
	if got := readFile(t, filepath.Join(dir, "Metroid.gba")); got != "0123456789" {
		t.Fatalf("resumed final = %q", got)
	}
}

func TestSameTitleDifferentROMsDoNotShareAFile(t *testing.T) {
	mount := t.TempDir()
	fetch := &fakeFetcher{fn: func(id int, off int64, w io.Writer, p func(int64)) error {
		return body(map[int]string{1: "first-bytes", 42: "second-bytes"}[id])(id, off, w, p)
	}}
	roms := []api.ROM{gba(1, "Zelda"), gba(42, "Zelda")}
	q := New(fetch, mount, roms)
	q.Run(context.Background())

	dir := partDir(mount)
	if got := readFile(t, filepath.Join(dir, "Zelda.gba")); got != "first-bytes" {
		t.Fatalf("Zelda.gba = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Zelda-42.gba")); got != "second-bytes" {
		t.Fatalf("Zelda-42.gba = %q", got)
	}

	// Run the same ROMs again. The clean name is taken, so ROM 1 now goes to
	// Zelda-1.gba (the clean-named file cannot be told apart from another
	// ROM's); ROM 42 was saved before and is skipped. Nothing is overwritten.
	fetch.calls = nil
	q2 := New(fetch, mount, roms)
	q2.Run(context.Background())
	if len(fetch.calls) != 1 || fetch.calls[0] != 1 {
		t.Fatalf("re-run fetched %v, want only ROM 1", fetch.calls)
	}
	if it := q2.Snapshot()[1]; it.Status != Done || !it.Skipped {
		t.Fatalf("ROM 42 item = %+v, want Done and Skipped", it)
	}
	if got := readFile(t, filepath.Join(dir, "Zelda.gba")); got != "first-bytes" {
		t.Fatalf("Zelda.gba overwritten: %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Zelda-42.gba")); got != "second-bytes" {
		t.Fatalf("Zelda-42.gba overwritten: %q", got)
	}
}

func TestIDFilenameInsertsIDBeforeExtensionAsOneSegment(t *testing.T) {
	if got := idFilename(gba(42, "Zelda")); got != "Zelda-42.gba" {
		t.Fatalf("idFilename = %q, want Zelda-42.gba", got)
	}
	got := idFilename(api.ROM{ID: 7, Title: "../x", Extension: ".gba"})
	if got != ".._x-7.gba" {
		t.Fatalf("idFilename = %q, want a sanitized single segment .._x-7.gba", got)
	}
}

func TestStopBeforeFirstByteAbortsRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		select { // never answers; the timeout only keeps a failing run from hanging
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()

	mount := t.TempDir()
	q := New(api.New(srv.URL, "tok", nil), mount, []api.ROM{gba(1, "Metroid")})
	done := make(chan struct{})
	go func() { q.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel before the first byte")
	}
	if exists(filepath.Join(partDir(mount), "Metroid.gba")) {
		t.Fatal("final file created")
	}
	if it := q.Snapshot()[0]; it.Status != Cancelled {
		t.Fatalf("item = %+v, want Cancelled", it)
	}
}

func gbaSized(id int, title string, size int64) api.ROM {
	r := gba(id, title)
	r.SizeBytes = &size
	return r
}

func TestAlreadyOnCardUnderTaggedNameIsSkipped(t *testing.T) {
	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Pokemon - Emerald Version (USA, Europe).gba"), []byte("abcdef"), 0o644)

	fetch := &fakeFetcher{fn: body("abcdef")}
	q := New(fetch, mount, []api.ROM{gbaSized(1, "Pokemon - Emerald Version", 6)})
	refreshed := false
	q.Refresh = func(string) error { refreshed = true; return nil }
	q.Run(context.Background())

	if len(fetch.calls) != 0 {
		t.Fatalf("fetched a ROM that is already on the card: %v", fetch.calls)
	}
	it := q.Snapshot()[0]
	if it.Status != Done || !it.Skipped || it.Existing != "Pokemon - Emerald Version (USA, Europe).gba" {
		t.Fatalf("item = %+v, want Done, Skipped, Existing = the card's file name", it)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || refreshed {
		t.Fatalf("a duplicate was written or Refresh ran: %d entries, refreshed=%v", len(entries), refreshed)
	}
}

func TestSimilarNameButDifferentSizeIsStillDownloaded(t *testing.T) {
	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Metroid (USA).gba"), []byte("1234"), 0o644)

	fetch := &fakeFetcher{fn: body("0123456789")}
	q := New(fetch, mount, []api.ROM{gbaSized(1, "Metroid", 10)})
	q.Run(context.Background())

	if len(fetch.calls) != 1 {
		t.Fatalf("a different dump was not downloaded: calls %v", fetch.calls)
	}
	if got := readFile(t, filepath.Join(dir, "Metroid (USA).gba")); got != "1234" {
		t.Fatalf("existing file touched: %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "Metroid.gba")); got != "0123456789" {
		t.Fatalf("Metroid.gba = %q", got)
	}
}

func TestSameSizeButDifferentNameOrExtensionIsStillDownloaded(t *testing.T) {
	mount := t.TempDir()
	dir := filepath.Join(mount, "Roms", "GBA")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Kirby.gba"), []byte("abcdef"), 0o644)   // same size, other game
	os.WriteFile(filepath.Join(dir, "Metroid.srm"), []byte("abcdef"), 0o644) // same name and size, a save file
	os.WriteFile(filepath.Join(dir, "Metroid.gba.part"), []byte("abcdef"), 0o644)

	fetch := &fakeFetcher{fn: body("abcdef")}
	q := New(fetch, mount, []api.ROM{gbaSized(1, "Metroid", 6)})
	q.Run(context.Background())

	if len(fetch.calls) != 1 {
		t.Fatalf("expected a download, calls %v", fetch.calls)
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"Pokemon - Emerald Version (USA, Europe)": "pokemon emerald version",
		"Donkey Kong 64 (U) [!]":                  "donkey kong 64",
		"Donkey_Kong_64_U_":                       "donkey kong 64 u",
		"  Super   Mario-Land  ":                  "super mario land",
		"(Unl)":                                   "",
	}
	for in, want := range cases {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
