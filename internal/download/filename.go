// Package download is the one-at-a-time ROM downloader: it maps each ROM to its
// muOS folder, writes it atomically (.part, Sync, rename) and resumes
// interrupted downloads.
package download

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

const (
	// maxNameBytes keeps "<name>.part" within the 255-byte limit of FAT/exFAT
	// and ext4 file names.
	maxNameBytes = 250
	maxExtBytes  = 32
)

// originalName is the sanitized, length-bounded original upload name of rom
// with room reserved for reserve extra bytes (an id suffix), or "" when rom has
// no usable one. The extension is kept whole; only the stem is truncated.
func originalName(rom api.ROM, reserve int) (stem, ext string) {
	if strings.TrimSpace(rom.Filename) == "" {
		return "", ""
	}
	name := sanitize(rom.Filename)
	ext = truncBytes(filepath.Ext(name), maxExtBytes)
	stem = strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
	stem = truncBytes(stem, maxNameBytes-len(ext)-reserve)
	return strings.TrimRight(stem, ". "), ext
}

// Filename builds the single-path-segment file name for rom. When the server
// reports the original upload name (rom.Filename) that is used, sanitized, so
// saves and box art keyed to the file name keep matching. Otherwise (old
// servers and rows) it is built from the title and extension (which includes
// the leading dot). Control characters are dropped;
// path separators and the characters FAT/exFAT cannot store are replaced with
// "_". An empty title becomes "rom-<id>". The result is never "." or "..".
func Filename(rom api.ROM) string {
	if stem, ext := originalName(rom, 0); stem != "" {
		return stem + ext
	}
	title := strings.TrimRight(strings.TrimSpace(sanitize(baseTitle(rom))), ". ")
	ext := truncBytes(sanitize(rom.Extension), maxExtBytes)
	title = truncBytes(title, maxNameBytes-len(ext))
	title = strings.TrimRight(title, ". ")
	if title == "" {
		title = fmt.Sprintf("rom-%d", rom.ID)
	}
	return title + ext
}

// baseTitle is rom.Title without a trailing copy of its own extension. Titles
// taken from file names can still end in ".z64"; without this the saved file
// would be "<title>.z64.z64". Only an extension equal to rom.Extension (any
// case) is dropped, so a title like "Disk 1.z64" on a .bin ROM is left alone.
func baseTitle(rom api.ROM) string {
	ext := rom.Extension
	if ext != "" && len(rom.Title) > len(ext) && strings.EqualFold(rom.Title[len(rom.Title)-len(ext):], ext) {
		return rom.Title[:len(rom.Title)-len(ext)]
	}
	return rom.Title
}

// idFilename is Filename with "-<id>" inserted before the extension
// ("Zelda.gba", id 42 -> "Zelda-42.gba"). It is used when the clean name is
// already taken by another ROM. It is still one sanitized path segment and,
// like Filename, prefers the original upload name.
func idFilename(rom api.ROM) string {
	suffix := fmt.Sprintf("-%d", rom.ID)
	if stem, ext := originalName(rom, len(suffix)); stem != "" {
		return stem + suffix + ext
	}
	title := strings.TrimRight(strings.TrimSpace(sanitize(baseTitle(rom))), ". ")
	ext := truncBytes(sanitize(rom.Extension), maxExtBytes)
	title = truncBytes(title, maxNameBytes-len(ext)-len(suffix))
	title = strings.TrimRight(title, ". ")
	if title == "" {
		return fmt.Sprintf("rom-%d%s", rom.ID, ext)
	}
	return title + suffix + ext
}

// sanitize drops control characters and replaces / \ and the FAT-reserved
// characters : * ? " < > | with "_".
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsControl(r):
			continue
		case strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// truncBytes cuts s to at most n bytes on a rune boundary.
func truncBytes(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// FormatBytes renders n as "512 B", "1.5 KB", "5.0 MB" or "3.5 GB".
func FormatBytes(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return fmt.Sprintf("%d B", n)
	case n < k*k:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < k*k*k:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	}
}
