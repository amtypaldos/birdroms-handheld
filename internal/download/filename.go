// Package download is the one-at-a-time ROM downloader: it maps each ROM to its
// muOS folder, writes it atomically (.part, Sync, rename) and resumes
// interrupted downloads.
package download

import (
	"fmt"
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

// Filename builds the single-path-segment file name for rom from its title and
// extension (which includes the leading dot). Control characters are dropped;
// path separators and the characters FAT/exFAT cannot store are replaced with
// "_". An empty title becomes "rom-<id>". The result is never "." or "..".
func Filename(rom api.ROM) string {
	title := strings.TrimRight(strings.TrimSpace(sanitize(rom.Title)), ". ")
	ext := truncBytes(sanitize(rom.Extension), maxExtBytes)
	title = truncBytes(title, maxNameBytes-len(ext))
	title = strings.TrimRight(title, ". ")
	if title == "" {
		title = fmt.Sprintf("rom-%d", rom.ID)
	}
	return title + ext
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
