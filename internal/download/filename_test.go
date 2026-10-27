package download

import (
	"strings"
	"testing"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
)

func TestFilename(t *testing.T) {
	cases := []struct {
		name string
		rom  api.ROM
		want string
	}{
		{"plain", api.ROM{ID: 1, Title: "Metroid Fusion", Extension: ".gba"}, "Metroid Fusion.gba"},
		{"slashes replaced", api.ROM{ID: 1, Title: `a/b\c`, Extension: ".gb"}, "a_b_c.gb"},
		{"control chars stripped", api.ROM{ID: 1, Title: "a\x00b\nc\x7fd", Extension: ".gb"}, "abcd.gb"},
		{"empty title uses id", api.ROM{ID: 42, Title: "", Extension: ".nes"}, "rom-42.nes"},
		{"title of only control chars uses id", api.ROM{ID: 7, Title: "\n\t", Extension: ".nes"}, "rom-7.nes"},
		{"dot-dot title without extension is not a traversal segment", api.ROM{ID: 5, Title: "..", Extension: ""}, "rom-5"},
		{"dot title without extension", api.ROM{ID: 6, Title: ".", Extension: ""}, "rom-6"},
		{"escape attempt stays one segment", api.ROM{ID: 1, Title: "../x", Extension: ".gba"}, ".._x.gba"},
		{"extension cannot carry a path", api.ROM{ID: 1, Title: "x", Extension: "/../.gba"}, "x_.._.gba"},
		{"fat-reserved characters replaced", api.ROM{ID: 1, Title: `Zelda: Link's "Awakening"? <DX> |*`, Extension: ".gbc"}, "Zelda_ Link's _Awakening__ _DX_ __.gbc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Filename(c.rom)
			if got != c.want {
				t.Fatalf("Filename = %q, want %q", got, c.want)
			}
			if strings.ContainsAny(got, `/\`) || got == "." || got == ".." {
				t.Fatalf("Filename %q is not a single safe path segment", got)
			}
		})
	}
}

func TestFilenameIsLengthBounded(t *testing.T) {
	got := Filename(api.ROM{ID: 1, Title: strings.Repeat("é", 300), Extension: ".gba"})
	if len(got)+len(".part") > 255 {
		t.Fatalf("name is %d bytes; with .part it exceeds the 255-byte limit", len(got))
	}
	if !strings.HasSuffix(got, ".gba") {
		t.Fatalf("extension lost: %q", got)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:               "0 B",
		999:             "999 B",
		1024:            "1.0 KB",
		1536:            "1.5 KB",
		5 * 1024 * 1024: "5.0 MB",
		3<<30 + 1<<29:   "3.5 GB",
	}
	for n, want := range cases {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
