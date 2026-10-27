# B1 — muOS device recon (RG40XX-V), 2026-10-27

Verified over SSH (`root`/`root` @ `muos.local`, password auth only; one transient "Permission denied" seen, retry works).

## Device / OS
- Board `rg40xx-v`, MustardOS **2606.0 ANDROMEDA**, kernel 4.9.170 aarch64, glibc 2.44 (Buildroot).
- Screen **640x480**, rotation 0, fb0 (virtual 640x960), ~59.98 Hz.
- IP on wlan0 (DHCP); `curl`/`wget` and system CA certs present.

## Storage
- `/mnt/sdcard` (mmcblk1p1, SD2, 128G): **ROMs at `/mnt/sdcard/Roms/<SYSTEM>`** (note `Roms`, not `ROMS`), plus `MUOS/{bios,save}`, `saves_RA`, `states_RA`, `ARCHIVE`, `BACKUP`.
- `/mnt/mmc` (mmcblk0p6, internal 50G): `MUOS/` (application, info, pickles, ...), `ARCHIVE`, `ROMS` (**empty**, unused).
- Runtime paths: `MUOS_STORE_DIR=/run/muos/storage` (bind mounts, e.g. `.../application` -> `/mnt/mmc/MUOS/application`). Read ROM mount at runtime via `GET_VAR device storage/rom/mount`.
- 55 system folders, upper-case names: A2600 A5200 A7800 A800 AMIGA APPS ATARI ATARIST ATOMISWAVE C64 CPS1 CPS2 CPS3 DOS DREAMCAST EASYRPG FBNEO FC FDS GB GBA GBC GG GW HBMAME LYNX MAME MD MDCD MSX N64 NAOMI NDS NEOGEO NEOCD NGP ONS OPENBOR PCE PCECD PGM2 PICO POKE PORTS PS PSP SATURN SCUMMVM SEGA32X SFC SMS VARCADE VB VIC20 WS.

## SDL / graphics
- Device ships **SDL2 2.28.5**, SDL2_ttf, SDL2_image, SDL2_mixer, SDL2_gfx, freetype, libdrm, Mali EGL/GLES in `/usr/lib`.
- Existing apps launch SDL with `SETUP_SDL_ENVIRONMENT`, `SDL_GAMECONTROLLERCONFIG_FILE=/usr/lib/gamecontrollerdb.txt`, and for GL `SDL_VIDEO_EGL_DRIVER=/usr/lib/libmali.so`, `SDL_OPENGL_ES_DRIVER=1`.
- sdl config: name `muOS-Keys`, blitter_disabled=1, scaler=0.

## Input
- Evdev: `muOS-Keys` (event3 / js1) is the gamepad; `muOS-Input-Source` (js0); `axp2202-pek` power key (event0).
- SDL controller map (guid `060000006d7500000b0700006f730000`): a:b2 b:b3 x:b4 y:b5 L:b6 R:b7 LT:b8 RT:b9 back:b10 start:b11 guide:b12, dpad = hat 0 (up 1, right 2, down 4, left 8), sticks a0-a3, volume up/down b1/b0.
- Use the SDL GameController API with the env var above rather than raw button numbers.

## Packaging (CORRECTION to plan)
- On 2606 apps are **`.muxapp`**, not `.muxzip`. `extract.sh` (`*.muxapp` case) extracts into `$MUOS_STORE_DIR/application` (i.e. `/mnt/mmc/MUOS/application/<AppName>/`). Drop it in an `ARCHIVE/` folder and install via the Archive Manager. The unsafe-path check rejects `..`/absolute entries.
- Layout of an app dir (from stock apps): `mux_launch.sh` (header comments `# HELP:`, `# ICON:`, `# GRID:`), `glyph/`, `mux_lang.ini`, plus the app payload (e.g. `.game/`).
- Stock launchers source `/opt/muos/script/var/stage.sh` + `retro.sh`, call `SETUP_STAGE_OVERLAY` and `SETUP_APP "<bin>" ""`, `cd` into the app dir, then run the binary; `CAFFEINE on/off` keeps the device awake.

## Not yet verified
- Whether a **fully static** Go/SDL binary can load SDL2 on device (cgo needs dynamic SDL2/glibc; likely build dynamically against the device's SDL2 with a glibc-2.44-compatible or older toolchain).
- Actual `.muxapp` internal zip layout (top-level folder vs. flat) — inspect a real one or install a test app.
- Game-list refresh after downloading a ROM (muOS appears to read the ROM dirs live; `MUOS/info/name`, `catalogue` hold metadata) — test empirically.
- Physical button -> SDL event mapping under a real SDL2 program (B2 "hello SDL" will confirm).
