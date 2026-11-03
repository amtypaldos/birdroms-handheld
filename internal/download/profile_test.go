package download

import (
	"context"
	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/firmware"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileDownloadUsesExactRoot(t *testing.T) {
	for _, known := range []bool{true, false} {
		root := t.TempDir()
		dirs := map[string]string{}
		folder := "_UNSORTED"
		if known {
			dirs["NES"] = "nes"
			folder = "nes"
		}
		p := firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, SystemDirs: dirs}
		q := NewWithProfile(&fakeFetcher{fn: body("data")}, p, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes"}})
		q.Run(context.Background())
		if got := readFile(t, filepath.Join(root, folder, "Mario.nes")); got != "data" {
			t.Fatal(got)
		}
		if exists(filepath.Join(root, "Roms")) {
			t.Fatal("appended muOS Roms")
		}
		if it := q.Snapshot()[0]; it.Status != Done || it.Known != known {
			t.Fatalf("%+v", it)
		}
	}
}

func TestProfileAlternateLibraryUsesSameMapping(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	size := int64(4)
	if err := os.Mkdir(filepath.Join(other, "nes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "nes", "Mario.nes"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	fetch := &fakeFetcher{fn: body("data")}
	q := NewWithProfile(fetch, firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, LibraryRoots: []string{other}, SystemDirs: map[string]string{"NES": "nes"}}, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes", SizeBytes: &size}})
	q.Run(context.Background())
	if it := q.Snapshot()[0]; it.Status != Done || !it.Skipped {
		t.Fatalf("%+v", it)
	}
	if len(fetch.calls) != 0 || exists(filepath.Join(root, "nes")) {
		t.Fatal("downloaded existing ROM")
	}
}

func TestProfileRemovedRootAndUnsafeMappingCannotDownload(t *testing.T) {
	for _, unsafe := range []bool{true, false} {
		root := t.TempDir()
		folder := "nes"
		if unsafe {
			folder = "../escape"
		} else {
			if err := os.Remove(root); err != nil {
				t.Fatal(err)
			}
		}
		fetch := &fakeFetcher{fn: body("data")}
		q := NewWithProfile(fetch, firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": folder}}, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes"}})
		q.Run(context.Background())
		if q.Snapshot()[0].Status != Failed || len(fetch.calls) != 0 {
			t.Fatal("unsafe download accepted")
		}
		if !unsafe && exists(root) {
			t.Fatal("recreated missing storage")
		}
	}
}

func TestProfileSystemSymlinkCannotEscape(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	if err := os.Symlink(out, filepath.Join(root, "nes")); err != nil {
		t.Fatal(err)
	}
	fetch := &fakeFetcher{fn: body("data")}
	q := NewWithProfile(fetch, firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": "nes"}}, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes"}})
	q.Run(context.Background())
	if q.Snapshot()[0].Status != Failed || len(fetch.calls) != 0 {
		t.Fatal("followed system symlink")
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}

func TestProfileDoesNotFollowPartSymlink(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nes"), 0755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(out, "keep")
	if err := os.WriteFile(victim, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(root, "nes", "Mario.nes.part")); err != nil {
		t.Fatal(err)
	}
	fetch := &fakeFetcher{fn: body("data")}
	q := NewWithProfile(fetch, firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": "nes"}}, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes"}})
	q.Run(context.Background())
	if readFile(t, victim) != "keep" {
		t.Fatal("partial-file symlink modified external file")
	}
	if q.Snapshot()[0].Status != Failed || len(fetch.calls) != 0 {
		t.Fatal("followed partial-file symlink")
	}
}

func TestProfileDetectsReplacedStorage(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "card")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	fetch := &fakeFetcher{fn: body("data")}
	q := NewWithProfile(fetch, firmware.Profile{Target: firmware.Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": "nes"}}, []api.ROM{{ID: 1, Title: "Mario", Platform: "NES", Extension: ".nes"}})
	if err := os.Rename(root, root+"-removed"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	q.Run(context.Background())
	if q.Snapshot()[0].Status != Failed || len(fetch.calls) != 0 {
		t.Fatal("downloaded to replacement mount point")
	}
}

func TestMissingMuOSRootCannotBecomeSymlink(t *testing.T) {
	mount := t.TempDir()
	fetch := &fakeFetcher{fn: body("data")}
	q := New(fetch, mount, []api.ROM{gba(1, "Mario")})
	outside := filepath.Join(mount, "other")
	if err := os.Mkdir(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other", filepath.Join(mount, "Roms")); err != nil {
		t.Fatal(err)
	}
	q.Run(context.Background())
	if q.Snapshot()[0].Status != Failed || len(fetch.calls) != 0 {
		t.Fatal("redirected new muOS ROM root")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("wrote outside configured root")
	}
}
