package platforms

import "testing"

func TestFolderKnownPlatforms(t *testing.T) {
	cases := map[string]string{
		"NES":                       "FC",
		"SNES":                      "SFC",
		"Nintendo 64":               "N64",
		"Game Boy":                  "GB",
		"Game Boy Color":            "GBC",
		"Game Boy Advance":          "GBA",
		"Nintendo DS":               "NDS",
		"Sega Master System":        "SMS",
		"Sega Genesis":              "MD",
		"Sega Game Gear":            "GG",
		"Sega 32X":                  "SEGA32X",
		"Sega Dreamcast":            "DREAMCAST",
		"Sony PlayStation":          "PS",
		"Sony PSP":                  "PSP",
		"PC Engine / TurboGrafx-16": "PCE",
		"Neo Geo Pocket":            "NGP",
		"Neo Geo Pocket Color":      "NGP",
		"WonderSwan":                "WS",
		"WonderSwan Color":          "WS",
		"Atari 2600":                "A2600",
		"Atari 7800":                "A7800",
		"Atari Lynx":                "LYNX",
	}
	for platform, want := range cases {
		dir, known := Folder(platform)
		if dir != want || !known {
			t.Errorf("Folder(%q) = (%q, %v), want (%q, true)", platform, dir, known, want)
		}
	}
}

func TestFolderUnknownGoesToUnsorted(t *testing.T) {
	for _, platform := range []string{
		"Nintendo GameCube", "Nintendo Wii", "Nintendo 3DS",
		"Disc Image", "Archive", "unknown", "", "Totally New Console",
		"../../etc", "a/b", "..", "GBA/..", `Game Boy\Advance`,
	} {
		dir, known := Folder(platform)
		if dir != "_UNSORTED" || known {
			t.Errorf("Folder(%q) = (%q, %v), want (\"_UNSORTED\", false)", platform, dir, known)
		}
	}
}

func TestFolderFromAliasTable(t *testing.T) {
	cases := map[string]string{
		"Sega CD / Mega-CD": "MDCD",
		"Neo Geo":           "NEOGEO",
		"Atari Jaguar":      "JAGUAR",
		"Virtual Boy":       "VB",
		"PICO-8":            "PICO",
	}
	for platform, want := range cases {
		dir, known := Folder(platform)
		if dir != want || !known {
			t.Errorf("Folder(%q) = (%q, %v), want (%q, true)", platform, dir, known, want)
		}
	}
}

func TestFolderNonGameIsUnsorted(t *testing.T) {
	dir, known := Folder("Apps")
	if dir != "_UNSORTED" || known {
		t.Errorf("Folder(\"Apps\") = (%q, %v), want (\"_UNSORTED\", false)", dir, known)
	}
}
