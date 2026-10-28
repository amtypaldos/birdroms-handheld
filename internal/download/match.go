package download

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

// normalizeName reduces a ROM title or file name (without extension) to a key
// for comparing "the same game" across naming conventions: region and dump
// tags in () and [] are dropped, case is folded, and every run of other
// non-alphanumeric characters becomes one space.
func normalizeName(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			if depth > 0 {
				depth--
			}
		case depth > 0:
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// findExisting looks in dir for a file that is very likely rom already: first
// one named like Filename(rom) (any case), then one with the same extension,
// the same normalized name and exactly the size the server reports. The
// normalized name may be that of the (clean) title or of the original file
// name, whose No-Intro form ("Zelda, The", scene numbers) the title drops.
// Without a known size nothing is ever treated as present (the caller keeps
// its conservative never-overwrite behaviour). It returns the file's name.
func findExisting(dir string, rom api.ROM) (string, bool) {
	if rom.SizeBytes == nil || *rom.SizeBytes <= 0 {
		return "", false
	}
	if strings.TrimSpace(rom.Filename) != "" {
		exact := Filename(rom)
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				name := e.Name()
				if !e.Type().IsRegular() || strings.HasPrefix(name, ".") || !strings.EqualFold(name, exact) {
					continue
				}
				if info, err := e.Info(); err == nil && info.Size() == *rom.SizeBytes {
					return name, true
				}
			}
		}
	}
	wants := map[string]bool{}
	if w := normalizeName(baseTitle(rom)); w != "" {
		wants[w] = true
	}
	if strings.TrimSpace(rom.Filename) != "" {
		orig := sanitize(rom.Filename)
		if w := normalizeName(strings.TrimSuffix(orig, filepath.Ext(orig))); w != "" {
			wants[w] = true
		}
	}
	if len(wants) == 0 {
		return "", false
	}
	ext := strings.ToLower(sanitize(rom.Extension))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || strings.HasPrefix(name, ".") {
			continue
		}
		if strings.ToLower(filepath.Ext(name)) != ext {
			continue
		}
		if !wants[normalizeName(strings.TrimSuffix(name, filepath.Ext(name)))] {
			continue
		}
		if info, err := e.Info(); err == nil && info.Size() == *rom.SizeBytes {
			return name, true
		}
	}
	return "", false
}
