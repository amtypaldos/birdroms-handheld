package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFileIsUnpaired(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "birdroms.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load(missing) error = %v, want os.ErrNotExist", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "birdroms.json")
	want := Config{ServerURL: "https://roms.example.com", DeviceToken: "tok-123"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != want {
		t.Fatalf("Load = %+v, want %+v", got, want)
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "birdroms.json")
	if err := Save(path, Config{ServerURL: "x", DeviceToken: "y"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestSaveMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes not meaningful on windows")
	}
	path := filepath.Join(t.TempDir(), "birdroms.json")
	if err := Save(path, Config{ServerURL: "x", DeviceToken: "y"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Overwrite an existing, more permissive file: mode must still end up 0600.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, Config{ServerURL: "x", DeviceToken: "z"}); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o, want 600", mode)
	}
}

func TestSaveJSONKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "birdroms.json")
	if err := Save(path, Config{ServerURL: "https://s", DeviceToken: "t"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m["server_url"] != "https://s" || m["device_token"] != "t" {
		t.Fatalf("unexpected JSON: %s", raw)
	}
}

func TestLoadCorruptFileIsDistinctError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "birdroms.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load(corrupt) = nil error")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt file must not look unpaired: %v", err)
	}
}

func TestPaired(t *testing.T) {
	if (Config{}).Paired() {
		t.Fatal("empty config reported paired")
	}
	if (Config{ServerURL: "x"}).Paired() {
		t.Fatal("config without token reported paired")
	}
	if !(Config{ServerURL: "x", DeviceToken: "y"}).Paired() {
		t.Fatal("full config not paired")
	}
}

func TestSaveFailsWhenParentIsAFile(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "notadir")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "birdroms.json")
	if err := Save(dest, Config{ServerURL: "https://x", DeviceToken: "t"}); err == nil {
		t.Fatal("Save under a file parent returned nil")
	}
	if b, _ := os.ReadFile(parent); string(b) != "x" {
		t.Fatalf("parent file changed: %q", b)
	}
	if _, err := os.Stat(dest); err == nil {
		t.Fatal("a config was created at the destination")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestWithSeedServerFillsOnlyAMissingURL(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "server.json")
	if err := os.WriteFile(seed, []byte(`{"server_url": "https://birds.example/"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := WithSeedServer(Config{}, seed); got.ServerURL != "https://birds.example/" {
		t.Fatalf("empty config got %q", got.ServerURL)
	}
	if got := WithSeedServer(Config{ServerURL: "https://mine.example"}, seed); got.ServerURL != "https://mine.example" {
		t.Fatalf("existing url overwritten: %q", got.ServerURL)
	}
	keep := Config{ServerURL: "", DeviceToken: "tok"}
	if got := WithSeedServer(keep, seed); got.DeviceToken != "tok" {
		t.Fatal("token lost")
	}
}

func TestWithSeedServerIgnoresMissingOrBrokenSeed(t *testing.T) {
	dir := t.TempDir()
	if got := WithSeedServer(Config{}, filepath.Join(dir, "nope.json")); got.ServerURL != "" {
		t.Fatalf("missing seed gave %q", got.ServerURL)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := WithSeedServer(Config{}, bad); got.ServerURL != "" {
		t.Fatalf("broken seed gave %q", got.ServerURL)
	}
}
