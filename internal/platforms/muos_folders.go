// Package platforms maps birdROMs platform names to muOS ROM folders.
package platforms

// Unsorted is the visible fallback folder for platforms with no known muOS
// folder. Files there are flagged in the UI so the user can move them.
const Unsorted = "_UNSORTED"

// folders holds only exact names from the verified muOS 2606 folder list
// (docs/b1-device-recon.md). The platform string is only ever a lookup key; it
// is never joined onto a path.
var folders = map[string]string{
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

// Folder returns the muOS folder name (a single path segment) for platform.
// Anything not in the table returns (Unsorted, false).
func Folder(platform string) (dir string, known bool) {
	if d, ok := folders[platform]; ok {
		return d, true
	}
	return Unsorted, false
}
