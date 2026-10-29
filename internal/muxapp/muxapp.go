// Package muxapp builds a muOS .muxapp archive (a zip) from a stage directory.
package muxapp

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Prefix is the folder every entry must live under. muOS 2606 runs
// `unzip -o <archive> -d $MUOS_STORE_DIR/application` for a .muxapp (see
// /opt/muos/script/mux/extract.sh) with no prefix stripping, so the app folder
// itself is the archive root: it lands in /mnt/mmc/MUOS/application/birdROMs/.
const Prefix = "birdROMs/"

var requiredEntries = []string{
	Prefix + "mux_launch.sh",
	Prefix + "mux_lang.ini",
	Prefix + "birdroms",
	Prefix + "glyph/birdroms.svg",
	Prefix + "assets/fonts/ChakraPetch-Bold.ttf",
	Prefix + "assets/fonts/OFL.txt",
	Prefix + "LICENSE",
	Prefix + "THIRD_PARTY_NOTICES.txt",
}

func modeFor(name string) fs.FileMode {
	if name == Prefix+"mux_launch.sh" || name == Prefix+"birdroms" {
		return 0o755
	}
	return 0o644
}

// validateName rejects entry names that are empty, absolute, contain "..",
// contain a backslash, or fall outside Prefix.
func validateName(name string) error {
	switch {
	case name == "":
		return errors.New("empty entry name")
	case strings.HasPrefix(name, "/"):
		return fmt.Errorf("absolute entry name %q", name)
	case strings.Contains(name, ".."):
		return fmt.Errorf("entry name %q contains ..", name)
	case strings.Contains(name, `\`):
		return fmt.Errorf("entry name %q contains a backslash", name)
	case !strings.HasPrefix(name, Prefix) || strings.HasSuffix(name, "/"):
		return fmt.Errorf("entry name %q is not a file under %s", name, Prefix)
	}
	return nil
}

// Build zips the regular files under stageDir into outPath. Nothing is
// written to outPath if any entry is invalid or a required one is missing.
func Build(stageDir, outPath string) error {
	var names []string
	err := filepath.WalkDir(stageDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stageDir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if err := validateName(name); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("entry %q is not a regular file", name)
		}
		names = append(names, name)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(names)

	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}
	for _, r := range requiredEntries {
		if !have[r] {
			return fmt.Errorf("stage is missing required entry %q", r)
		}
	}
	if len(names) != len(requiredEntries) {
		for _, n := range names {
			known := false
			for _, r := range requiredEntries {
				known = known || n == r
			}
			if !known {
				return fmt.Errorf("unexpected entry %q", n)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".muxapp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := writeZip(tmp, stageDir, names); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), outPath)
}

func writeZip(w io.Writer, stageDir string, names []string) error {
	zw := zip.NewWriter(w)
	for _, name := range names {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.SetMode(modeFor(name))
		ew, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(stageDir, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		_, err = io.Copy(ew, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
