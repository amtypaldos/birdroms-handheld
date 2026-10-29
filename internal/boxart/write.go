package boxart

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Saver writes box art for downloaded ROMs. The zero value does nothing
// useful; set the fields.
type Saver struct {
	// CatalogueDirs are candidate muOS "info/catalogue" directories, most
	// preferred first (on the device /run/muos/storage/info/catalogue, then
	// each storage's MUOS/info/catalogue). The first that exists is used.
	CatalogueDirs []string
	// ContentDir is muOS's content config directory (/opt/muos/share/info/content),
	// where per-folder core.cfg and per-ROM cfg files say which catalogue a
	// ROM uses. Missing is fine.
	ContentDir string
	// Fetch returns the cover image bytes for a cover URL.
	Fetch func(coverURL string) ([]byte, error)
}

// Result says what Save did. Exactly one of Path (art written) or Skip (the
// reason nothing was written; not an error) is set.
type Result struct {
	Path string
	Skip string
}

func skip(format string, a ...any) Result { return Result{Skip: fmt.Sprintf(format, a...)} }

// Save writes the cover for the ROM file fileName that now sits in muOS ROM
// folder `folder` (e.g. "SFC"). fileName must be the name the ROM has on the
// card, because muOS finds art by that name. Existing art is never replaced.
// An error means the cover could not be fetched or stored; a Skip is normal.
func (s *Saver) Save(folder, fileName, coverURL string) (Result, error) {
	if coverURL == "" {
		return skip("ROM has no cover"), nil
	}
	if !ValidName(folder) || fileName != filepath.Base(fileName) || !ValidName(fileName) {
		return skip("unsafe folder or file name"), nil
	}
	stem := Stem(fileName)
	if stem == "" {
		return skip("empty file name stem"), nil
	}
	root := s.catalogueRoot()
	if root == "" {
		return skip("no muOS catalogue directory on this machine"), nil
	}

	cat, src := Resolve(Inputs{
		Folder:    folder,
		ROMCfg:    s.readContent(folder, stem+".cfg"),
		FolderCfg: s.readContent(folder, "core.cfg"),
		Assign:    readAssign(filepath.Join(filepath.Dir(root), "manifest", "assign.json")),
	})
	if src == FromNone {
		return skip("no catalogue assigned to %s", folder), nil
	}
	catDir := filepath.Join(root, cat)
	if src == FromAssign {
		// Not assigned in muOS yet: only use a catalogue folder muOS already
		// created, never invent one.
		if st, err := os.Stat(catDir); err != nil || !st.IsDir() {
			return skip("catalogue %q does not exist", cat), nil
		}
	}
	boxDir := filepath.Join(catDir, "box")
	if hasArt(boxDir, stem) || hasArt(filepath.Join(boxDir, "640x480"), stem) {
		return skip("art already present"), nil
	}

	raw, err := s.Fetch(coverURL)
	if err != nil {
		return Result{}, fmt.Errorf("fetch cover: %w", err)
	}
	data, ext, err := Prepare(raw)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(boxDir, 0o755); err != nil {
		return Result{}, err
	}
	final := filepath.Join(boxDir, stem+"."+ext)
	if err := writeAtomic(final, data); err != nil {
		return Result{}, err
	}
	return Result{Path: final}, nil
}

func (s *Saver) catalogueRoot() string {
	for _, d := range s.CatalogueDirs {
		if d == "" {
			continue
		}
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}

func (s *Saver) readContent(folder, name string) string {
	if s.ContentDir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(s.ContentDir, folder, name))
	if err != nil {
		return ""
	}
	return string(b)
}

func readAssign(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func hasArt(dir, stem string) bool {
	des, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	names := make([]string, 0, len(des))
	for _, d := range des {
		if !d.IsDir() {
			names = append(names, d.Name())
		}
	}
	return HasArt(names, stem)
}

// writeAtomic writes data to a dot-named temp file in final's directory
// (muOS ignores dot files), syncs it and renames it into place, unless final
// appeared in the meantime.
func writeAtomic(final string, data []byte) error {
	tmp := filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".part")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		if _, serr := os.Stat(final); serr == nil {
			err = errors.New("boxart: art appeared while writing; left alone")
		}
	}
	if err == nil {
		err = os.Rename(tmp, final)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// Default returns a Saver for the muOS device layout. mounts are the ROM
// storages (BIRDROMS_ROM_MOUNTS plus the download mount); fetch gets covers.
// BIRDROMS_CATALOGUE_DIR and BIRDROMS_CONTENT_DIR override the locations.
func Default(mounts []string, getenv func(string) string, fetch func(string) ([]byte, error)) *Saver {
	s := &Saver{ContentDir: "/opt/muos/share/info/content", Fetch: fetch}
	if d := getenv("BIRDROMS_CONTENT_DIR"); d != "" {
		s.ContentDir = d
	}
	if d := getenv("BIRDROMS_CATALOGUE_DIR"); d != "" {
		s.CatalogueDirs = []string{d}
		return s
	}
	s.CatalogueDirs = []string{"/run/muos/storage/info/catalogue"}
	for _, m := range mounts {
		if m = strings.TrimSpace(m); m != "" {
			s.CatalogueDirs = append(s.CatalogueDirs, filepath.Join(m, "MUOS", "info", "catalogue"))
		}
	}
	return s
}
