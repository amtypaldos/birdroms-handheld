// Package boxart saves a downloaded ROM's cover where muOS looks for box art:
// <storage>/info/catalogue/<catalogue name>/box/<ROM file name without
// extension>.<image ext>. This file is the pure decision logic (which
// catalogue, which name, skip rules); write.go does the I/O.
//
// What muOS does (MustardOS/frontend, common/content/catalogue.c and
// core/common.c): the catalogue name of a ROM comes from the content config
// under /opt/muos/share/info/content/<ROM folder>/, line 4 of a per-ROM
// "<ROM stem>.cfg" if there is one, else line 3 of the folder's "core.cfg".
// A folder with no assignment has an empty catalogue name and shows no art.
// The image is looked up by the ROM's file name without its last extension,
// case-insensitively, in box/<WxH>/ and then box/.
package boxart

import (
	"path/filepath"
	"strings"
)

// Line numbers (1-based) of the catalogue name in muOS content configs.
const (
	FolderCatalogueLine = 3 // <folder>/core.cfg: core, system, catalogue, ...
	ROMCatalogueLine    = 4 // <folder>/<stem>.cfg: name, core, system, catalogue, ...
)

// imageExts are the extensions muOS loads for box art. Matching is
// case-insensitive.
var imageExts = []string{"svg", "png", "apng", "jpg", "jpeg", "webp", "qoi", "tga", "gif", "bmp", "pcx"}

// Stem returns name without its last extension, as muOS does for the art
// lookup (a name with several dots loses only the last one). A leading dot is
// not an extension.
func Stem(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return name
	}
	return name[:i]
}

// CfgLine returns the trimmed 1-based line n of a muOS content config, or ""
// when the file is shorter.
func CfgLine(content string, n int) string {
	if n < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if n > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[n-1])
}

// ValidName reports whether s is safe to use as a single catalogue folder
// name. Config files are data, never trusted as paths.
func ValidName(s string) bool {
	if strings.TrimSpace(s) == "" || strings.HasPrefix(s, ".") {
		return false
	}
	return !strings.ContainsAny(s, "/\\\x00")
}

// Source says where a catalogue name came from.
type Source int

const (
	FromNone      Source = iota // no catalogue could be determined
	FromROMCfg                  // the ROM's own cfg (assigned to this ROM only)
	FromFolderCfg               // the folder's core.cfg (assigned to the folder)
	FromAssign                  // muOS's folder-name lookup table (not assigned yet)
)

// Inputs is everything needed to pick a catalogue. ROMCfg and FolderCfg are
// the contents of the matching config files ("" when absent); Assign is
// muOS's info/manifest/assign.json (folder key -> catalogue name), may be nil.
type Inputs struct {
	Folder    string
	ROMCfg    string
	FolderCfg string
	Assign    map[string]string
}

// Resolve picks the catalogue folder name for a ROM the way muOS does: the
// per-ROM assignment, then the folder's, and, when the folder has not been
// assigned a core yet, muOS's own folder-name table (the one its auto-assign
// uses). It returns ("", FromNone) when nothing valid is found.
func Resolve(in Inputs) (string, Source) {
	if n := CfgLine(in.ROMCfg, ROMCatalogueLine); ValidName(n) {
		return n, FromROMCfg
	}
	if n := CfgLine(in.FolderCfg, FolderCatalogueLine); ValidName(n) {
		return n, FromFolderCfg
	}
	for _, key := range []string{in.Folder, assignKey(in.Folder)} {
		if n, ok := in.Assign[key]; ok && ValidName(n) {
			return n, FromAssign
		}
	}
	return "", FromNone
}

// assignKey is the form assign.json uses for a folder: lower case letters
// and digits only ("Roms/SFC" -> "sfc").
func assignKey(folder string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(folder) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// HasArt reports whether entries (file names of a box directory) already hold
// an image for stem. The match is case-insensitive and ignores dot files and
// non-image files, like muOS's.
func HasArt(entries []string, stem string) bool {
	if stem == "" {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e, ".") {
			continue
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(e)), ".")
		for _, x := range imageExts {
			if ext == x && strings.EqualFold(strings.TrimSuffix(e, filepath.Ext(e)), stem) {
				return true
			}
		}
	}
	return false
}
