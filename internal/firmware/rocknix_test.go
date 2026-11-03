package firmware

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rocknixEnv(root string, extra map[string]string) func(string) string {
	return func(k string) string {
		if value, ok := extra[k]; ok {
			return value
		}
		if k == "BIRDROMS_ROM_ROOT" {
			return root
		}
		return ""
	}
}

func TestResolveRocknixUsesVerifiedSystemDirectories(t *testing.T) {
	root := t.TempDir()
	p, err := Resolve(Rocknix, rocknixEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if p.Target != Rocknix || p.ROMRoot != root || p.ExternalArt != "none" || p.UsesMuOSArt() {
		t.Fatalf("profile: %+v", p)
	}
	for name, want := range map[string]string{"NES": "nes", "SNES": "snes", "Game Boy": "gb", "Game Boy Color": "gbc", "Game Boy Advance": "gba", "Sega Genesis": "genesis", "Sony PlayStation": "psx", "Neo Geo CD": "neocd", "PC Engine SuperGrafx": "sgfx"} {
		if got, known := p.Folder(name); got != want || !known {
			t.Errorf("%s: %q %v, want %q", name, got, known, want)
		}
	}
	for _, name := range []string{"unknown", "../../etc", "Nintendo Switch", "FC", "SFC"} {
		if got, known := p.Folder(name); got != "_UNSORTED" || known {
			t.Errorf("%s did not use safe fallback: %s %v", name, got, known)
		}
	}
	muos := MuOSProfile(root)
	if got, _ := muos.Folder("NES"); got != "FC" {
		t.Fatal("ROCKNIX affected muOS map")
	}
}

func TestRocknixOverridesAndAlternateROMRoots(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	p, err := Resolve(Rocknix, rocknixEnv(root, map[string]string{
		"BIRDROMS_ROM_ROOTS":     root + string(os.PathListSeparator) + other + string(os.PathListSeparator) + other,
		"BIRDROMS_PLATFORM_DIRS": `{"Sega Genesis":"megadrive","Custom Platform":"custom/games"}`,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LibraryRoots) != 1 || p.LibraryRoots[0] != other {
		t.Fatalf("library roots: %v", p.LibraryRoots)
	}
	for key, want := range map[string]string{"Sega Genesis": "megadrive", "Custom Platform": "custom/games", "NES": "nes"} {
		if got, known := p.Folder(key); got != want || !known {
			t.Errorf("override %s: %s %v", key, got, known)
		}
	}
}

func TestRocknixRejectsMalformedOrUnsafeOverrides(t *testing.T) {
	root := t.TempDir()
	for _, raw := range []string{`[]`, `null`, `{"NES":null}`, `{"NES":4}`, `{"NES":"../escape"}`, `{"NES":"/tmp/escape"}`, `{"NES":"a/../escape"}`, `{"NES":"a\\b"}`, `{"NES":""}`, `{"NES":"."}`, `{"":"nes"}`, `{"NES":"nes","NES":"other"}`, `{"NES":"nes"} {}`, `{"NES":"nes"`} {
		if _, err := Resolve(Rocknix, rocknixEnv(root, map[string]string{"BIRDROMS_PLATFORM_DIRS": raw})); err == nil {
			t.Errorf("accepted override %s", raw)
		}
	}
}

func TestRocknixRequiresExistingExplicitROMRoot(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "absent")
	for _, root := range []string{"", "/", "relative", missing} {
		_, err := Resolve(Rocknix, rocknixEnv(root, map[string]string{"BIRDROMS_ROM_MOUNT": parent}))
		if err == nil {
			t.Errorf("accepted root %q", root)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("created absent ROM storage")
	}
	if _, err := Resolve(Rocknix, rocknixEnv(parent, map[string]string{"BIRDROMS_ROM_ROOTS": "relative"})); err == nil {
		t.Fatal("accepted relative alternate root")
	}
}

func TestRocknixMapsAreIndependentPerResolution(t *testing.T) {
	root := t.TempDir()
	p, err := Resolve(Rocknix, rocknixEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	p.SystemDirs["NES"] = "changed"
	p2, err := Resolve(Rocknix, rocknixEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := p2.Folder("NES"); got != "nes" {
		t.Fatal("shared mutable mappings")
	}
}

func TestRocknixOverrideLimit(t *testing.T) {
	root := t.TempDir()
	if _, err := Resolve(Rocknix, rocknixEnv(root, map[string]string{"BIRDROMS_PLATFORM_DIRS": strings.Repeat(" ", 65537)})); err == nil {
		t.Fatal("unbounded override input")
	}
}
