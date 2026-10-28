package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

func TestFilenameUsesOriginalFilename(t *testing.T) {
	cases := []struct {
		name string
		rom  api.ROM
		want string
	}{
		{"original kept", api.ROM{ID: 1, Title: "Bomberman 64", Extension: ".z64", Filename: "Bomberman 64 (U) [!].z64"}, "Bomberman 64 (U) [!].z64"},
		{"separators sanitized", api.ROM{ID: 1, Title: "x", Extension: ".gb", Filename: "a:b/c.gb"}, "a_b_c.gb"},
		{"only an extension falls back to title", api.ROM{ID: 1, Title: "Metroid", Extension: ".gba", Filename: ".gba"}, "Metroid.gba"},
		{"blank falls back to title", api.ROM{ID: 1, Title: "Metroid", Extension: ".gba", Filename: "  "}, "Metroid.gba"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Filename(c.rom); got != c.want {
				t.Fatalf("Filename = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFilenameOriginalTruncationKeepsExtension(t *testing.T) {
	rom := api.ROM{ID: 1, Extension: ".z64", Filename: strings.Repeat("a", 400) + ".z64"}
	got := Filename(rom)
	if len(got) > maxNameBytes || !strings.HasSuffix(got, ".z64") {
		t.Fatalf("Filename = %d bytes %q, want <= %d ending .z64", len(got), got, maxNameBytes)
	}
}

func TestIDFilenameUsesOriginalFilename(t *testing.T) {
	rom := api.ROM{ID: 7, Title: "Bomberman 64", Extension: ".z64", Filename: "Bomberman 64 (U) [!].z64"}
	if got, want := idFilename(rom), "Bomberman 64 (U) [!]-7.z64"; got != want {
		t.Fatalf("idFilename = %q, want %q", got, want)
	}
}

func TestFindExistingMatchesOriginalFilename(t *testing.T) {
	dir := t.TempDir()
	size := int64(5)
	rom := api.ROM{ID: 1, Title: "Other Title", Extension: ".z64", Filename: "Bomberman 64 (U) [!].z64", SizeBytes: &size}
	if err := os.WriteFile(filepath.Join(dir, "bomberman 64 (u) [!].Z64"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := findExisting(dir, rom); !ok || got != "bomberman 64 (u) [!].Z64" {
		t.Fatalf("findExisting = %q, %v", got, ok)
	}
	wrong := int64(6)
	rom.SizeBytes = &wrong
	if got, ok := findExisting(dir, rom); ok {
		t.Fatalf("wrong size matched %q", got)
	}
}

func TestFindExistingMatchesNoIntroFormOfOriginalFilename(t *testing.T) {
	cases := []struct {
		name string
		rom  api.ROM
		card string
	}{
		{"moved article", api.ROM{ID: 1, Title: "The Legend of Zelda - The Minish Cap", Filename: "Legend of Zelda, The - The Minish Cap (USA).gba", Extension: ".gba"}, "Legend of Zelda, The - The Minish Cap (Europe).gba"},
		{"scene number", api.ROM{ID: 2, Title: "Pokemon Black 2", Filename: "6111 - Pokemon Black 2 (US) (frieNDS).nds", Extension: ".nds"}, "6111 - Pokemon Black 2 (US).nds"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			size := int64(5)
			c.rom.SizeBytes = &size
			if err := os.WriteFile(filepath.Join(dir, c.card), []byte("12345"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got, ok := findExisting(dir, c.rom); !ok || got != c.card {
				t.Fatalf("findExisting = %q, %v, want %q", got, ok, c.card)
			}
			wrong := int64(6)
			c.rom.SizeBytes = &wrong
			if got, ok := findExisting(dir, c.rom); ok {
				t.Fatalf("wrong size matched %q", got)
			}
		})
	}
}

func TestFindExistingEmptyFilenameUsesTitleOnly(t *testing.T) {
	dir := t.TempDir()
	size := int64(5)
	rom := api.ROM{ID: 3, Title: "Metroid", Extension: ".gba", Filename: "  ", SizeBytes: &size}
	if err := os.WriteFile(filepath.Join(dir, "Metroid (USA).gba"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := findExisting(dir, rom); !ok || got != "Metroid (USA).gba" {
		t.Fatalf("findExisting = %q, %v", got, ok)
	}
}
