package firmware

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// resolveDirectories builds an independent profile using complete ROM roots.
func resolveDirectories(target Target, folders map[string]string, getenv func(string) string) (Profile, error) {
	p := Profile{Target: target, ROMRoot: getenv("BIRDROMS_ROM_ROOT"), ExternalArt: "none", SystemDirs: make(map[string]string, len(folders))}
	for name, dir := range folders {
		p.SystemDirs[name] = dir
	}
	for _, root := range filepath.SplitList(getenv("BIRDROMS_ROM_ROOTS")) {
		if root == "" || root == p.ROMRoot {
			continue
		}
		duplicate := false
		for _, existing := range p.LibraryRoots {
			if existing == root {
				duplicate = true
				break
			}
		}
		if !duplicate {
			p.LibraryRoots = append(p.LibraryRoots, root)
		}
	}
	if raw := getenv("BIRDROMS_PLATFORM_DIRS"); raw != "" {
		if err := applyDirectoryOverrides(p.SystemDirs, raw); err != nil {
			return Profile{}, fmt.Errorf("BIRDROMS_PLATFORM_DIRS: %w", err)
		}
	}
	return p, p.Validate()
}

// Use token decoding to reject duplicate keys and null/non-string values rather
// than letting encoding/json silently accept a last-wins or empty override.
func applyDirectoryOverrides(dirs map[string]string, raw string) error {
	if len(raw) > 65536 {
		return fmt.Errorf("mapping JSON exceeds 64 KiB")
	}
	d := json.NewDecoder(strings.NewReader(raw))
	start, err := d.Token()
	if err != nil {
		return fmt.Errorf("invalid mapping JSON")
	}
	if start != json.Delim('{') {
		return fmt.Errorf("mapping must be an object")
	}
	seen := make(map[string]bool)
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("invalid mapping key")
		}
		key, ok := tok.(string)
		if !ok || strings.TrimSpace(key) == "" || seen[key] {
			return fmt.Errorf("mapping keys must be nonempty and unique")
		}
		seen[key] = true
		value, err := d.Token()
		if err != nil {
			return fmt.Errorf("invalid mapping value")
		}
		dir, ok := value.(string)
		if !ok || !safeDir(dir) {
			return fmt.Errorf("mapping directories must be safe relative paths")
		}
		dirs[key] = dir
	}
	end, err := d.Token()
	if err != nil || end != json.Delim('}') {
		return fmt.Errorf("incomplete mapping object")
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing mapping data")
	}
	return nil
}
