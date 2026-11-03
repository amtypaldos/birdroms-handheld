package firmware

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// rocknixFolders is independent of muOS's alias CSV. Paths were checked against
// official ROCKNIX system pages on 2026-11-03 (https://rocknix.org/systems/).
// The page slugs match the folder except neocd (neogeocd), ngpc (ngp), sgfx
// (supergrafx), pcenginecd (pcengine), fbneo (fbn), and the SNES variants (snes).
// The distribution's rocknix-systems BIOS script also confirms GB/GBC/GBA, NDS,
// PSP, Dreamcast, Game Gear, Master System, Sega CD, Saturn and PSX folder names:
// https://github.com/ROCKNIX/distribution/blob/93c889bb853f2a11e51cecb0a9a75221219a2cce/projects/ROCKNIX/packages/rocknix/sources/scripts/rocknix-systems
// These folders do not imply that a particular handheld can emulate a system.
var rocknixFolders = map[string]string{
	"Amiga":                        "amiga",
	"Atari 2600":                   "atari2600",
	"Atari 5200":                   "atari5200",
	"Atari 7800":                   "atari7800",
	"Atari 800":                    "atari800",
	"Atari Jaguar":                 "atarijaguar",
	"Atari Lynx":                   "atarilynx",
	"Famicom Disk System":          "fds",
	"Game & Watch":                 "gameandwatch",
	"Game Boy":                     "gb",
	"Game Boy Advance":             "gba",
	"Game Boy Color":               "gbc",
	"Neo Geo":                      "neogeo",
	"Neo Geo CD":                   "neocd",
	"Neo Geo Pocket":               "ngp",
	"Neo Geo Pocket Color":         "ngpc",
	"NES":                          "nes",
	"Nintendo 64":                  "n64",
	"Nintendo DS":                  "nds",
	"PC Engine / TurboGrafx-16":    "pcengine",
	"PC Engine CD / TurboGrafx-CD": "pcenginecd",
	"PC Engine SuperGrafx":         "sgfx",
	"PICO-8":                       "pico-8/carts",
	"Satellaview (BS-X)":           "satellaview",
	"ScummVM":                      "scummvm",
	"Sega 32X":                     "sega32x",
	"Sega CD / Mega-CD":            "segacd",
	"Sega Dreamcast":               "dreamcast",
	"Sega Game Gear":               "gamegear",
	"Sega Genesis":                 "genesis",
	"Sega Master System":           "mastersystem",
	"Sega Saturn":                  "saturn",
	"Sega SG-1000":                 "sg-1000",
	"SNES":                         "snes",
	"Sony PlayStation":             "psx",
	"Sony PSP":                     "psp",
	"Sufami Turbo":                 "sufami",
	"Virtual Boy":                  "virtualboy",
	"WonderSwan":                   "wonderswan",
	"WonderSwan Color":             "wonderswancolor",
	"Capcom CPS-1":                 "cps1",
	"Capcom CPS-2":                 "cps2",
	"Capcom CPS-3":                 "cps3",
	"FinalBurn Neo":                "fbneo",
	"MAME":                         "mame",
}

func resolveRocknix(getenv func(string) string) (Profile, error) {
	p := Profile{Target: Rocknix, ROMRoot: getenv("BIRDROMS_ROM_ROOT"), ExternalArt: "none", SystemDirs: make(map[string]string, len(rocknixFolders))}
	for name, dir := range rocknixFolders {
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
		if err := applyRocknixOverrides(p.SystemDirs, raw); err != nil {
			return Profile{}, fmt.Errorf("BIRDROMS_PLATFORM_DIRS: %w", err)
		}
	}
	return p, p.Validate()
}

// Use token decoding to reject duplicate keys and null/non-string values rather
// than letting encoding/json silently accept a last-wins or empty override.
func applyRocknixOverrides(dirs map[string]string, raw string) error {
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
