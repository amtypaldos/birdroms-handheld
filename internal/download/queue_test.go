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

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

// fakeFetcher runs fn for each download and records the calls.
type fakeFetcher struct {
	fn    func(id int, offset int64, w io.Writer, progress func(int64)) error
	calls []int
}

func (f *fakeFetcher) DownloadROMFrom(id int, offset int64, w io.Writer, progress func(int64)) error {
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
