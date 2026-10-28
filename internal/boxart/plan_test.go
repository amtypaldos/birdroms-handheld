package boxart

import "testing"

func TestStem(t *testing.T) {
	cases := map[string]string{
		"Super Mario World (U).smc":   "Super Mario World (U)",
		"Mr. Do! (USA).nes":           "Mr. Do! (USA)",
		"Pokémon – Émeraude (FR).gba": "Pokémon – Émeraude (FR)",
		"Game.v1.2.final.gb":          "Game.v1.2.final",
		"archive.tar.gz":              "archive.tar",
		"NoExtension":                 "NoExtension",
		".hidden":                     ".hidden",
		"日本語のゲーム (Japan).sfc":         "日本語のゲーム (Japan)",
		"Title-123.gba":               "Title-123",
		"":                            "",
	}
	for in, want := range cases {
		if got := Stem(in); got != want {
			t.Errorf("Stem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCfgLine(t *testing.T) {
	folder := "snes9x_libretro.so\nNintendo SNES - SFC\nNintendo SNES - SFC\n0\nmu-snes9x"
	rom := "Super Mario World (U)\nsnes9x2010_libretro.so\nNintendo SNES - SFC\nSNES Special\n0\nmu-snes9x 2010\n/mnt/sdcard/ROMS/\nSFC\nSuper Mario World (U).smc"
	if got := CfgLine(folder, FolderCatalogueLine); got != "Nintendo SNES - SFC" {
		t.Errorf("folder cfg: %q", got)
	}
	if got := CfgLine(rom, ROMCatalogueLine); got != "SNES Special" {
		t.Errorf("rom cfg: %q", got)
	}
	if got := CfgLine("a\r\nb\r\nc \r\n", 3); got != "c" {
		t.Errorf("CRLF/space: %q", got)
	}
	if got := CfgLine("one\ntwo", 3); got != "" {
		t.Errorf("short file: %q", got)
	}
	if got := CfgLine("x", 0); got != "" {
		t.Errorf("line 0: %q", got)
	}
}

func TestValidCatalogueName(t *testing.T) {
	good := []string{"Nintendo SNES - SFC", "Sega Mega Drive - Genesis", "PICO-8", "2048", "Atari ST-STE-TT-Falcon"}
	bad := []string{"", ".", "..", "../x", "a/b", `a\b`, "a\x00b", ".hidden", " ", "Folder/../x"}
	for _, n := range good {
		if !ValidName(n) {
			t.Errorf("ValidName(%q) = false", n)
		}
	}
	for _, n := range bad {
		if ValidName(n) {
			t.Errorf("ValidName(%q) = true", n)
		}
	}
}

func TestResolve(t *testing.T) {
	assign := map[string]string{"sfc": "Nintendo SNES - SFC", "gba": "Nintendo Game Boy Advance", "bennugd": "BennuGD", "2600": "Atari 2600"}
	cases := []struct {
		name string
		in   Inputs
		want string
		src  Source
	}{
		{"rom cfg wins", Inputs{Folder: "SFC", ROMCfg: "n\nc\ns\nSpecial\n0", FolderCfg: "c\ns\nFolderCat\n0", Assign: assign}, "Special", FromROMCfg},
		{"folder cfg", Inputs{Folder: "SFC", FolderCfg: "c\ns\nFolderCat\n0", Assign: assign}, "FolderCat", FromFolderCfg},
		{"blank rom line falls through", Inputs{Folder: "SFC", ROMCfg: "n\nc\ns\n\n0", FolderCfg: "c\ns\nFolderCat\n0"}, "FolderCat", FromFolderCfg},
		{"assign fallback", Inputs{Folder: "SFC", Assign: assign}, "Nintendo SNES - SFC", FromAssign},
		{"assign exact case first", Inputs{Folder: "BennuGD", Assign: assign}, "BennuGD", FromAssign},
		{"assign digits", Inputs{Folder: "2600", Assign: assign}, "Atari 2600", FromAssign},
		{"assign punctuation stripped", Inputs{Folder: "G-B_A", Assign: assign}, "Nintendo Game Boy Advance", FromAssign},
		{"unknown", Inputs{Folder: "_UNSORTED", Assign: assign}, "", FromNone},
		{"no data", Inputs{Folder: "SFC"}, "", FromNone},
		{"invalid cfg name rejected, falls through", Inputs{Folder: "SFC", FolderCfg: "c\ns\n../../etc\n0", Assign: assign}, "Nintendo SNES - SFC", FromAssign},
		{"invalid everywhere", Inputs{Folder: "X", FolderCfg: "c\ns\n..\n0", Assign: map[string]string{"x": "a/b"}}, "", FromNone},
	}
	for _, c := range cases {
		got, src := Resolve(c.in)
		if got != c.want || src != c.src {
			t.Errorf("%s: Resolve = (%q, %v), want (%q, %v)", c.name, got, src, c.want, c.src)
		}
	}
}

func TestHasArt(t *testing.T) {
	entries := []string{"Zelda (USA).PNG", "Other.jpg", "notes.txt", "Mario.png.tmp", ".Hidden.png"}
	yes := []string{"Zelda (USA)", "zelda (usa)", "Other"}
	no := []string{"Mario", "notes", "Hidden", "Zelda", ""}
	for _, s := range yes {
		if !HasArt(entries, s) {
			t.Errorf("HasArt(%q) = false", s)
		}
	}
	for _, s := range no {
		if HasArt(entries, s) {
			t.Errorf("HasArt(%q) = true", s)
		}
	}
}
