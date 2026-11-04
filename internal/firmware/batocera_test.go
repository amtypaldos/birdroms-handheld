package firmware_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/download"
	"github.com/amtypaldos/birdroms-handheld/internal/firmware"
)

func batoceraEnv(root string, extra map[string]string) func(string) string {
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

func TestBatoceraUsesVerifiedDistinctDirectories(t *testing.T) {
	root := t.TempDir()
	p, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if p.Target != firmware.Batocera || p.ROMRoot != root || p.ExternalArt != "none" || p.UsesMuOSArt() {
		t.Fatalf("profile: %+v", p)
	}
	wantDirs := map[string]string{"NES": "nes", "SNES": "snes", "Game Boy": "gb", "Game Boy Color": "gbc", "Game Boy Advance": "gba", "Sega Genesis": "megadrive", "Sega CD / Mega-CD": "megacd", "Sony PlayStation": "psx", "Neo Geo CD": "neogeocd", "PC Engine SuperGrafx": "supergrafx", "Atari Lynx": "lynx", "Sega SG-1000": "sg1000", "WonderSwan": "wswan", "WonderSwan Color": "wswanc", "PICO-8": "pico8"}
	for name, want := range wantDirs {
		if got, known := p.Folder(name); got != want || !known {
			t.Errorf("%s: %q %v, want %q", name, got, known, want)
		}
	}
	rocknix, err := firmware.Resolve(firmware.Rocknix, batoceraEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if dir, _ := rocknix.Folder("Sega Genesis"); dir != "genesis" {
		t.Fatal("Batocera changed ROCKNIX mapping")
	}
	if dir, _ := firmware.MuOSProfile(root).Folder("NES"); dir != "FC" {
		t.Fatal("Batocera changed muOS mapping")
	}
	for _, name := range []string{"unknown", "../../etc", "FC", "SFC", "Capcom CPS-1", "HBMAME"} {
		if got, known := p.Folder(name); known || got != "_UNSORTED" {
			t.Fatalf("unmapped %s: %s %v", name, got, known)
		}
	}
}

func TestBatoceraOverridesAndAlternateRoots(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	p, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, map[string]string{"BIRDROMS_ROM_ROOTS": root + string(os.PathListSeparator) + other + string(os.PathListSeparator) + other, "BIRDROMS_PLATFORM_DIRS": `{"Amiga":"amiga1200","Custom Platform":"custom/games"}`}))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.LibraryRoots) != 1 || p.LibraryRoots[0] != other {
		t.Fatalf("alternate roots: %v", p.LibraryRoots)
	}
	for name, want := range map[string]string{"Amiga": "amiga1200", "Custom Platform": "custom/games", "Sega Genesis": "megadrive"} {
		if got, known := p.Folder(name); !known || got != want {
			t.Fatalf("%s: %s %v", name, got, known)
		}
	}
	p.SystemDirs["Sega Genesis"] = "changed"
	fresh, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	if dir, _ := fresh.Folder("Sega Genesis"); dir != "megadrive" {
		t.Fatal("shared mutable maps")
	}
}

func TestBatoceraRejectsUnsafeOverrides(t *testing.T) {
	root := t.TempDir()
	for _, raw := range []string{`[]`, `null`, `{"NES":null}`, `{"NES":4}`, `{"NES":"../escape"}`, `{"NES":"/tmp/escape"}`, `{"NES":"a/../escape"}`, `{"NES":"a\\b"}`, `{"NES":""}`, `{"NES":"."}`, `{"":"nes"}`, `{"NES":"nes","NES":"other"}`, `{"NES":"nes"} {}`, `{"NES":"nes"`, strings.Repeat(" ", 65537)} {
		if _, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, map[string]string{"BIRDROMS_PLATFORM_DIRS": raw})); err == nil {
			t.Errorf("accepted unsafe override %q", raw)
		}
	}
}

func TestBatoceraRequiresExplicitPresentWritableRoot(t *testing.T) {
	parent := t.TempDir()
	missing := filepath.Join(parent, "missing")
	for _, root := range []string{"", "/", "relative", missing} {
		if _, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, map[string]string{"BIRDROMS_ROM_MOUNT": parent})); err == nil {
			t.Errorf("accepted root %q", root)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("created absent storage")
	}
	if _, err := firmware.Resolve(firmware.Batocera, batoceraEnv(parent, map[string]string{"BIRDROMS_ROM_ROOTS": "relative"})); err == nil {
		t.Fatal("accepted relative alternate root")
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(parent, 0555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(parent, 0755)
		if _, err := firmware.Resolve(firmware.Batocera, batoceraEnv(parent, nil)); err == nil {
			t.Fatal("accepted unwritable storage")
		}
	}
}

type profileFetcher struct{ calls int }

func (f *profileFetcher) DownloadROMFrom(_ context.Context, _ int, _ int64, w io.Writer, progress func(int64)) error {
	f.calls++
	n, err := io.WriteString(w, "rom")
	if progress != nil {
		progress(int64(n))
	}
	return err
}

func TestBatoceraDownloadDoesNotAppendMuOSRoms(t *testing.T) {
	root := t.TempDir()
	p, err := firmware.Resolve(firmware.Batocera, batoceraEnv(root, nil))
	if err != nil {
		t.Fatal(err)
	}
	fetch := &profileFetcher{}
	q := download.NewWithProfile(fetch, p, []api.ROM{{ID: 1, Title: "Game", Filename: "Game (USA).md", Platform: "Sega Genesis", Extension: ".md"}})
	q.Run(context.Background())
	data, err := os.ReadFile(filepath.Join(root, "megadrive", "Game (USA).md"))
	if err != nil || string(data) != "rom" {
		t.Fatalf("wrong download path/data: %q %v, status %+v", data, err, q.Snapshot())
	}
	if _, err := os.Stat(filepath.Join(root, "Roms")); !os.IsNotExist(err) {
		t.Fatal("appended muOS Roms directory")
	}
	if q.Snapshot()[0].Status != download.Done || fetch.calls != 1 {
		t.Fatal("download did not complete")
	}
}
