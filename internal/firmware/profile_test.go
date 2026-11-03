package firmware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMuOSCompatibility(t *testing.T) {
	mount, other := t.TempDir(), t.TempDir()
	env := map[string]string{"BIRDROMS_ROM_MOUNT": mount, "BIRDROMS_ROM_MOUNTS": mount + string(os.PathListSeparator) + other}
	p, err := Resolve("", func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if p.Target != MuOS || p.ROMRoot != filepath.Join(mount, "Roms") || p.ExternalArt != "muos" {
		t.Fatalf("profile: %+v", p)
	}
	if d, known := p.Folder("NES"); !known || d != "FC" {
		t.Fatalf("NES: %s %v", d, known)
	}
	if len(p.LibraryRoots) != 1 || p.LibraryRoots[0] != filepath.Join(other, "Roms") {
		t.Fatalf("libraries: %v", p.LibraryRoots)
	}
}

func TestResolveDoesNotInventPlannedFirmwareSupport(t *testing.T) {
	for _, target := range []Target{"typo", Batocera, Knulli} {
		if _, err := Resolve(target, func(string) string { return "" }); err == nil {
			t.Errorf("accepted unavailable target %q", target)
		}
	}
}

func TestValidateRootAndMappings(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"../escape", "/absolute", "a/../../b", "a\\b", ".", "a/../b", ""} {
		p := Profile{Target: Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": dir}}
		if err := p.Validate(); err == nil {
			t.Errorf("accepted mapping %q", dir)
		}
	}
	for _, bad := range []string{"", "relative", filepath.Join(root, "missing")} {
		if err := (Profile{Target: Rocknix, ROMRoot: bad}).Validate(); err == nil {
			t.Errorf("accepted root %q", bad)
		}
	}
	if err := (Profile{Target: Rocknix, ROMRoot: root, SystemDirs: map[string]string{"NES": "nes"}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMissingMountIsNotCreated(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := Resolve(MuOS, func(k string) string {
		if k == "BIRDROMS_ROM_MOUNT" {
			return missing
		}
		return ""
	})
	if err == nil {
		t.Fatal("missing mount accepted")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("missing mount was created")
	}
}

func TestFolderFallbackAndInvalidMapping(t *testing.T) {
	p := Profile{Target: Rocknix, SystemDirs: map[string]string{"NES": "nes", "bad": "../escape"}}
	for _, key := range []string{"unknown", "../../escape", "bad"} {
		if d, known := p.Folder(key); d != "_UNSORTED" || known {
			t.Fatalf("%s: %s %v", key, d, known)
		}
	}
}

func TestMuOSMappingIsNotMutableAcrossProfiles(t *testing.T) {
	mount := t.TempDir()
	env := func(k string) string {
		if k == "BIRDROMS_ROM_MOUNT" {
			return mount
		}
		return ""
	}
	p, err := Resolve(MuOS, env)
	if err != nil {
		t.Fatal(err)
	}
	p.SystemDirs["NES"] = "other"
	other, err := Resolve(MuOS, env)
	if err != nil {
		t.Fatal(err)
	}
	if dir, _ := other.Folder("NES"); dir != "FC" {
		t.Fatal("shared mutable mappings")
	}
}

func TestExternalArtIsMuOSOnly(t *testing.T) {
	for _, target := range []Target{MuOS, Rocknix, Batocera, Knulli} {
		p := Profile{Target: target, ExternalArt: "muos"}
		if p.UsesMuOSArt() != (target == MuOS) {
			t.Fatalf("wrong art for %s", target)
		}
	}
}

func TestReadOnlyStorage(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("run as non-root to test real write permissions")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(root, 0755)
	if err := (Profile{Target: Rocknix, ROMRoot: root}).Validate(); err == nil {
		t.Fatal("read-only root accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("left write probe")
	}
}

func TestMuOSRejectsFilesystemRootAsCard(t *testing.T) {
	if _, err := Resolve(MuOS, func(k string) string {
		if k == "BIRDROMS_ROM_MOUNT" {
			return "/"
		}
		return ""
	}); err == nil {
		t.Fatal("accepted system root as ROM card")
	}
	if err := MuOSProfile("/").Validate(); err == nil {
		t.Fatal("accepted /Roms on system root")
	}
}
