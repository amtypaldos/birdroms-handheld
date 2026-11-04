package firmware

// rocknixFolders is independent of muOS's alias CSV. Paths were checked against
// official ROCKNIX system pages on 2026-11-03 (https://rocknix.org/systems/).
// The page slugs match the folder except neocd (neogeocd), ngpc (ngp), sgfx
// (supergrafx), pcenginecd (pcengine), fbneo (fbn), and the SNES variants (snes).
// The distribution's rocknix-systems BIOS script also confirms GB/GBC/GBA, NDS,
// PSP, Dreamcast, Game Gear, Master System, Sega CD, Saturn and PSX folder names:
// https://github.com/ROCKNIX/distribution/blob/93c889bb853f2a11e51cecb0a9a75221219a2cce/projects/ROCKNIX/packages/rocknix/sources/scripts/rocknix-systems
// These folders do not imply that a particular handheld can emulate a system.
var rocknixFolders = map[string]string{
	"Amiga":                        "amiga",
	"Atari 2600":                   "atari2600",
	"Atari 5200":                   "atari5200",
	"Atari 7800":                   "atari7800",
	"Atari 800":                    "atari800",
	"Atari Jaguar":                 "atarijaguar",
	"Atari Lynx":                   "atarilynx",
	"Famicom Disk System":          "fds",
	"Game & Watch":                 "gameandwatch",
	"Game Boy":                     "gb",
	"Game Boy Advance":             "gba",
	"Game Boy Color":               "gbc",
	"Neo Geo":                      "neogeo",
	"Neo Geo CD":                   "neocd",
	"Neo Geo Pocket":               "ngp",
	"Neo Geo Pocket Color":         "ngpc",
	"NES":                          "nes",
	"Nintendo 64":                  "n64",
	"Nintendo DS":                  "nds",
	"PC Engine / TurboGrafx-16":    "pcengine",
	"PC Engine CD / TurboGrafx-CD": "pcenginecd",
	"PC Engine SuperGrafx":         "sgfx",
	"PICO-8":                       "pico-8/carts",
	"Satellaview (BS-X)":           "satellaview",
	"ScummVM":                      "scummvm",
	"Sega 32X":                     "sega32x",
	"Sega CD / Mega-CD":            "segacd",
	"Sega Dreamcast":               "dreamcast",
	"Sega Game Gear":               "gamegear",
	"Sega Genesis":                 "genesis",
	"Sega Master System":           "mastersystem",
	"Sega Saturn":                  "saturn",
	"Sega SG-1000":                 "sg-1000",
	"SNES":                         "snes",
	"Sony PlayStation":             "psx",
	"Sony PSP":                     "psp",
	"Sufami Turbo":                 "sufami",
	"Virtual Boy":                  "virtualboy",
	"WonderSwan":                   "wonderswan",
	"WonderSwan Color":             "wonderswancolor",
	"Capcom CPS-1":                 "cps1",
	"Capcom CPS-2":                 "cps2",
	"Capcom CPS-3":                 "cps3",
	"FinalBurn Neo":                "fbneo",
	"MAME":                         "mame",
}

func resolveRocknix(getenv func(string) string) (Profile, error) {
	return resolveDirectories(Rocknix, rocknixFolders, getenv)
}
