package muxapp

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const prefix = "birdROMs/"

var required = []string{
	prefix + "assets/fonts/ChakraPetch-Bold.ttf",
	prefix + "birdroms",
	prefix + "glyph/birdroms.svg",
	prefix + "mux_lang.ini",
	prefix + "mux_launch.sh",
}

func writeStageFile(t *testing.T, stage, name string) {
	t.Helper()
	p := filepath.Join(stage, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately create everything 0600/0644 so the builder, not the stage, decides modes.
	if err := os.WriteFile(p, []byte("stand-in "+name), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newStage(t *testing.T) string {
	t.Helper()
	stage := t.TempDir()
	for _, n := range required {
		writeStageFile(t, stage, n)
	}
	return stage
}

func TestBuildWritesExactlyTheRequiredEntries(t *testing.T) {
	stage := newStage(t)
	out := filepath.Join(t.TempDir(), "dist", "birdROMs.muxapp")

	if err := Build(stage, out); err != nil {
		t.Fatalf("Build: %v", err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	var got []string
	for _, f := range zr.File {
		got = append(got, f.Name)
	}
	sort.Strings(got)
	want := append([]string(nil), required...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries = %v, want %v", got, want)
		}
	}
}

func TestBuildSetsExecutableBitOnlyOnLauncherAndBinary(t *testing.T) {
	stage := newStage(t)
	out := filepath.Join(t.TempDir(), "birdROMs.muxapp")
	if err := Build(stage, out); err != nil {
		t.Fatalf("Build: %v", err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		want := os.FileMode(0o644)
		if f.Name == prefix+"mux_launch.sh" || f.Name == prefix+"birdroms" {
			want = 0o755
		}
		if got := f.Mode().Perm(); got != want {
			t.Errorf("%s mode = %o, want %o", f.Name, got, want)
		}
	}
}

func TestBuildRejectsUnsafeNamesAndWritesNoZip(t *testing.T) {
	cases := map[string]string{
		"dotdot in name":    prefix + "evil..name",
		"dotdot dir":        prefix + "a../b",
		"outside prefix":    "stray.txt",
		"other app folder":  "other/mux_launch.sh",
		"backslash in name": prefix + `bad\name`,
	}
	for label, name := range cases {
		t.Run(label, func(t *testing.T) {
			stage := newStage(t)
			writeStageFile(t, stage, name)
			out := filepath.Join(t.TempDir(), "birdROMs.muxapp")

			if err := Build(stage, out); err == nil {
				t.Fatal("Build succeeded, want error")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("zip was written (stat err = %v), want none", err)
			}
		})
	}
}

func TestBuildRejectsMissingRequiredEntry(t *testing.T) {
	stage := newStage(t)
	if err := os.Remove(filepath.Join(stage, filepath.FromSlash(prefix+"birdroms"))); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "birdROMs.muxapp")
	if err := Build(stage, out); err == nil {
		t.Fatal("Build succeeded without birdroms, want error")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("zip was written (stat err = %v), want none", err)
	}
}

func TestValidateName(t *testing.T) {
	bad := []string{"", "/abs/path", "birdROMs/../x", "birdROMs/a..b", `birdROMs\x`, "other/x", "birdROMs/", "application/birdROMs/birdroms"}
	for _, n := range bad {
		if err := validateName(n); err == nil {
			t.Errorf("validateName(%q) = nil, want error", n)
		}
	}
	if err := validateName(prefix + "birdroms"); err != nil {
		t.Errorf("validateName(valid) = %v", err)
	}
}
