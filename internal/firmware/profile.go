// Package firmware describes ROM storage independently of a firmware's launcher.
package firmware

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amtypaldos/birdroms-handheld/internal/platforms"
)

type Target string

const (
	MuOS     Target = "muos"
	Rocknix  Target = "rocknix"
	Batocera Target = "batocera"
	Knulli   Target = "knulli"
)

// Profile uses complete ROM roots, not card mounts. SystemDirs is indexed by
// canonical server platform names; raw server strings are never path components.
// Profiles are copied by queues and must not be changed while in use.
type Profile struct {
	Target       Target
	ROMRoot      string
	LibraryRoots []string
	SystemDirs   map[string]string
	ExternalArt  string
}

// MuOSProfile adapts the legacy launcher contract without changing its paths.
func MuOSProfile(mount string, libraries ...string) Profile {
	p := Profile{Target: MuOS, SystemDirs: platforms.Folders(), ExternalArt: "muos"}
	if mount != "" {
		p.ROMRoot = filepath.Join(mount, "Roms")
	}
	for _, lib := range libraries {
		if lib != "" && lib != mount {
			p.LibraryRoots = append(p.LibraryRoots, filepath.Join(lib, "Roms"))
		}
	}
	return p
}

// Resolve defaults an unset target to muOS. Planned targets fail closed until
// their launcher, directory map and update contract are implemented together.
func Resolve(target Target, getenv func(string) string) (Profile, error) {
	if target == "" {
		target = MuOS
	}
	if target != MuOS {
		return Profile{}, fmt.Errorf("firmware %q is not available in this build", target)
	}
	mount := getenv("BIRDROMS_ROM_MOUNT")
	if filepath.Clean(mount) == string(filepath.Separator) {
		return Profile{}, fmt.Errorf("system root cannot be a ROM card")
	}
	if err := writableDirectory(mount); err != nil {
		return Profile{}, fmt.Errorf("BIRDROMS_ROM_MOUNT: %w", err)
	}
	p := MuOSProfile(mount, filepath.SplitList(getenv("BIRDROMS_ROM_MOUNTS"))...)
	return p, p.Validate()
}

func safeDir(s string) bool {
	return s != "" && s != "." && !filepath.IsAbs(s) && filepath.Clean(s) == s &&
		!strings.ContainsAny(s, "\\\x00") && s != ".." && !strings.HasPrefix(s, ".."+string(filepath.Separator))
}

// Folder returns a safe mapped directory or the visible unknown-system fallback.
func (p Profile) Folder(platform string) (string, bool) {
	if dir, ok := p.SystemDirs[platform]; ok && safeDir(dir) {
		return dir, true
	}
	return platforms.Unsorted, false
}

// Validate never creates absent storage roots. A new muOS Roms directory is
// allowed only when the launcher-provided card mount already exists.
func (p Profile) Validate() error {
	switch p.Target {
	case MuOS, Rocknix, Batocera, Knulli:
	default:
		return fmt.Errorf("unknown firmware %q", p.Target)
	}
	if p.ExternalArt != "" && p.ExternalArt != "none" && !(p.Target == MuOS && p.ExternalArt == "muos") {
		return fmt.Errorf("invalid external art profile")
	}
	for key, dir := range p.SystemDirs {
		if !safeDir(dir) {
			return fmt.Errorf("unsafe system directory for %q", key)
		}
	}
	root := p.ROMRoot
	if p.Target == MuOS && filepath.Dir(root) == string(filepath.Separator) {
		return fmt.Errorf("muOS ROM root must be on a card mount")
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || root == string(filepath.Separator) {
		return fmt.Errorf("invalid ROM root")
	}
	if p.Target == MuOS && filepath.Base(root) == "Roms" {
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			root = filepath.Dir(root)
		}
	}
	if err := writableDirectory(root); err != nil {
		return fmt.Errorf("ROM storage: %w", err)
	}
	for _, lib := range p.LibraryRoots {
		if lib == "" || !filepath.IsAbs(lib) || filepath.Clean(lib) != lib {
			return fmt.Errorf("invalid library root")
		}
	}
	return nil
}

// A real create/close/remove catches read-only mounts, ACLs and permissions
// without assuming FAT/exFAT implements chmod. No probe is left on success.
func writableDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return fmt.Errorf("storage directory is not set or absolute")
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("storage is not a directory")
	}
	f, err := os.CreateTemp(path, ".birdroms-write-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

// UsesMuOSArt prevents another firmware from using muOS catalogue paths.
func (p Profile) UsesMuOSArt() bool { return p.Target == MuOS && p.ExternalArt == "muos" }
