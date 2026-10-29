package update

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/download"
)

func TestNeedsUpdate(t *testing.T) {
	if NeedsUpdate("abc", "abc", true) {
		t.Fatal("same hash should not update")
	}
	if !NeedsUpdate("", "abc", true) {
		t.Fatal("missing stamp should update once")
	}
	if !NeedsUpdate("old", "new", true) {
		t.Fatal("different hash should update")
	}
	if NeedsUpdate("", "abc", false) || NeedsUpdate("old", "", true) {
		t.Fatal("unavailable or empty hash should not update")
	}
}

func TestStageWritesNewBinaryAndSkipsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	pkg := muxapp(t, map[string]string{
		"birdROMs/birdroms":                          "binary",
		"birdROMs/mux_launch.sh":                     "launch",
		"birdROMs/server.json":                       `{"server_url":"https://birds.example"}`,
		"birdROMs/config.json":                       "wipe",
		"birdROMs/assets/fonts/ChakraPetch-Bold.ttf": "font",
	})

	if err := Stage(bytes.NewReader(pkg), int64(len(pkg)), dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, NewBinary)); got != "binary" {
		t.Fatalf("staged binary = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "birdroms")); !os.IsNotExist(err) {
		t.Fatal("stage replaced the running binary")
	}
	if got := read(t, filepath.Join(dir, "config.json")); got != "keep" {
		t.Fatalf("config.json = %q, want keep", got)
	}
	if got := read(t, filepath.Join(dir, "mux_launch.sh")); got != "launch" {
		t.Fatalf("launcher = %q", got)
	}
	if got := read(t, filepath.Join(dir, "assets/fonts/ChakraPetch-Bold.ttf")); got != "font" {
		t.Fatalf("font = %q", got)
	}
}

func TestStageRejectsPathsOutsideThePackage(t *testing.T) {
	dir := t.TempDir()
	pkg := muxapp(t, map[string]string{
		"birdROMs/../evil":  "no",
		"birdROMs/birdroms": "binary",
	})
	if err := Stage(bytes.NewReader(pkg), int64(len(pkg)), dir); err == nil {
		t.Fatal("expected a path refusal")
	}
	if _, err := os.Stat(filepath.Join(dir, NewBinary)); !os.IsNotExist(err) {
		t.Fatal("failed stage left birdroms.new")
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "evil")); err == nil {
		t.Fatal("zip wrote outside the app dir")
	}
}

func TestJobStagesOnSuccessAndCleansUpOnCancel(t *testing.T) {
	dir := t.TempDir()
	body := muxapp(t, map[string]string{"birdROMs/birdroms": "new", "birdROMs/mux_launch.sh": "sh"})
	job := NewJob(fakeDownload{body: body}, dir, "abc", int64(len(body)))
	job.Run(context.Background())

	if !Ready(dir) {
		t.Fatal("expected a staged update")
	}
	if got := read(t, filepath.Join(dir, NewStamp)); got != "abc" {
		t.Fatalf("stamp = %q", got)
	}
	snap := job.Snapshot()
	if len(snap) != 1 || snap[0].Status != download.Done || snap[0].ROM.Title != "Updating birdROMs" {
		t.Fatalf("snapshot = %+v", snap)
	}

	cancelDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := NewJob(fakeDownload{body: body, err: ctx.Err()}, cancelDir, "abc", 10)
	cancelled.Run(ctx)
	if Ready(cancelDir) {
		t.Fatal("cancelled update left a staged binary")
	}
}

type fakeDownload struct {
	body []byte
	err  error
}

func (f fakeDownload) DownloadHandheld(ctx context.Context, w io.Writer, progress func(int64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	n, err := w.Write(f.body)
	if progress != nil {
		progress(int64(n))
	}
	return err
}

func muxapp(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, contents := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestJobFailureRemovesPartial(t *testing.T) {
	dir := t.TempDir()
	job := NewJob(fakeDownload{err: errors.New("nope")}, dir, "abc", 10)
	job.Run(context.Background())
	if Ready(dir) {
		t.Fatal("failed update left a staged binary")
	}
	if job.Snapshot()[0].Status != download.Failed {
		t.Fatalf("status = %s", job.Snapshot()[0].Status)
	}
}

func TestStageRejectsEntriesOutsideBirdROMsRoot(t *testing.T) {
	for _, bad := range []string{"evil", "other/birdroms", "/etc/passwd", "birdROMs/a/../../escape", "birdROMs\\x"} {
		dir := t.TempDir()
		files := map[string]string{"birdROMs/birdroms": "binary", "birdROMs/mux_launch.sh": "launch"}
		files[bad] = "no"
		pkg := muxapp(t, files)
		err := Stage(bytes.NewReader(pkg), int64(len(pkg)), dir)
		if err == nil {
			t.Errorf("%q: expected refusal", bad)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("%q: rejected package still wrote %d file(s)", bad, len(entries))
		}
	}
}

func TestStageNeedsABinary(t *testing.T) {
	dir := t.TempDir()
	pkg := muxapp(t, map[string]string{"birdROMs/mux_launch.sh": "launch"})
	if err := Stage(bytes.NewReader(pkg), int64(len(pkg)), dir); err == nil {
		t.Fatal("expected an error for a package without the binary")
	}
	if _, err := os.Stat(filepath.Join(dir, "mux_launch.sh")); !os.IsNotExist(err) {
		t.Fatal("wrote files from a package with no binary")
	}
}

func TestStageReplacesLauncherWithoutRewritingItsInode(t *testing.T) {
	dir := t.TempDir()
	launcher := filepath.Join(dir, "mux_launch.sh")
	if err := os.WriteFile(launcher, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	running, err := os.Open(launcher) // stands in for the shell still reading it
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	pkg := muxapp(t, map[string]string{"birdROMs/birdroms": "b", "birdROMs/mux_launch.sh": "new"})
	if err := Stage(bytes.NewReader(pkg), int64(len(pkg)), dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, launcher); got != "new" {
		t.Fatalf("launcher = %q", got)
	}
	old, _ := io.ReadAll(running)
	if string(old) != "old" {
		t.Fatalf("open launcher now reads %q; it must keep the old content", old)
	}
	if info, _ := os.Stat(launcher); info.Mode().Perm()&0o100 == 0 {
		t.Fatal("launcher lost its exec bit")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(leftovers) != 0 {
		t.Fatalf("temp files left: %v", leftovers)
	}
}

func TestReadStamp(t *testing.T) {
	dir := t.TempDir()
	if got, err := ReadStamp(dir); err != nil || got != "" {
		t.Fatalf("missing stamp = %q, %v", got, err)
	}
	os.WriteFile(filepath.Join(dir, StampName), []byte("abc\n"), 0o644)
	if got, _ := ReadStamp(dir); got != "abc" {
		t.Fatalf("stamp = %q", got)
	}
}

func TestJobFailureLeavesNoStagedFiles(t *testing.T) {
	dir := t.TempDir()
	body := []byte("this is not a zip")
	NewJob(fakeDownload{body: body}, dir, "abc", int64(len(body))).Run(context.Background())
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("bad package left %d file(s) behind", len(entries))
	}
}
