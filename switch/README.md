# birdROMs for Nintendo Switch

Native C++17/libnx/SDL2 port of the existing Go handheld client, built alongside
muOS/ROCKNIX. This homebrew downloads your birdROMs library to RetroArch ROM
folders. It is not an emulator or Switch title installer.

**Build and automated Linux tests pass. The user reports the application fully
working on a physical Switch after the pairing fixes.** The checklist below
remains available for broader device and failure-case testing.

## Versioning

Releases follow the repository convention: `vYYYY.MM.DD`, with `.1`, `.2`, etc.
for additional releases on the same date. The Switch NACP version and HTTP user
agent use the same version without the leading `v`. `v2026.11.01` replaces the
initial Switch-only `switch-v0.1.0` naming; older tags remain available.

## Build

From the repository root on Linux with Docker access:

```sh
./switch/scripts/build.sh -j2
python3 switch/scripts/validate-nro.py
python3 switch/scripts/package.py
./switch/scripts/test.sh
```

Outputs: `switch/birdroms.nro` and `switch/birdROMs-switch.zip`, each with a SHA-256 file. The ZIP contains the NRO and its version/hash manifest; the self-hosted server adds its token-free server seed. The Dockerfile pins the official devkitPro
`devkita64` image by digest, including devkitA64, libnx, SDL2, SDL2_image,
SDL2_ttf, curl, minizip, zlib, image codecs and font libraries. No host compiler or
privileged host package installation is required. Docker must be accessible to
your user. The first pull is large. Tests use a separate Debian container with
native GCC, curl development files and Python; no Switch hardware is needed.

With an existing devkitPro install, install the official `switch-dev`,
`switch-sdl2`, `switch-sdl2_image`, `switch-sdl2_ttf`, `switch-minizip`, and `switch-curl` packages,
set `DEVKITPRO`/`DEVKITA64`, then `make -C switch -j2`. Use the pinned container
for the dependency versions used for this artifact. The Makefile uses
`pkg-config` for static SDL dependencies and libnx's `switch_rules`.

For a clean build: `./switch/scripts/build.sh clean`, followed by the normal
build command. Host tests directly: `cd switch && sh tests/run.sh` with GCC,
libcurl/minizip/zlib development headers/libraries and Python 3 installed.

## Install and pair

1. Copy `birdroms.nro` to `/switch/birdroms/birdroms.nro` on the SD card.
2. Launch it in Homebrew Menu under Atmosphère. No custom sysmodule is needed.
3. On first launch, A opens the Switch software keyboard for your server URL
   (for example `https://roms.example.org`). Alternatively create the config
   file below on a computer. Use the canonical URL: HTTP redirects are rejected.
4. Scan the displayed QR with your phone, sign into birdROMs, and approve the
   device. The code and approval URL are also displayed. The device polls for
   approval and stores the returned token on SD.
5. Open a platform or playlist, select ROMs and press Plus. Downloads go to
   `/retroarch/roms/<mapped-folder>/` by default.

Full-memory Homebrew Menu mode (title override) is recommended for large
libraries and covers. Album/applet mode has a smaller memory budget and may
not provide the software keyboard; prefill `server_url` if needed. The user has confirmed operation on physical hardware;
mode-specific limitations and the full checklist still need broader testing. The interface renders at 1280×720 in
handheld or docked mode. No touchscreen support is implemented.

## Self-hosted installation and automatic updates

The server's Handheld page now offers a Nintendo Switch ZIP. Extract it to the
SD-card root, keeping `switch/birdroms/` intact. It contains `birdroms.nro`,
`version.json` and a `server.json` naming that server. There is no device token
or runtime `config.json` in the archive. The app reads the server seed into
memory on first launch and writes config only after receiving a pairing token.
An existing server URL/config takes precedence over a new seed.

After pairing, the app checks its paired server in the background on launch.
A newer date-versioned Switch package downloads and is verified before the app
exits to install it. It restarts when the loader supports that; otherwise reopen
it from Homebrew Menu. **B** cancels/skips updates for the current launch;
**ZL on home** checks again. Failed updates stop with a readable error until an
explicit retry, and offline/unavailable servers do not prevent browsing.

Updating preserves `sdmc:/switch/birdroms/config.json`, the token, ROM base,
platform mappings and ROM files. The JSON remains writable using the existing
backup/recovery save path. The downloaded package's settings never overwrite it.
Only the verified NRO is staged for installation. The manifest's NRO size/hash
is checked against authenticated server metadata; the whole personalized ZIP
hash is deliberately not compared to the unpersonalized published ZIP hash.

Updates require launching the actual `.nro` from SD. The app does not guess an
installation target for unsupported/netloader launches. It closes SDL/font and
ROMFS handles before replacing its executable, preserves a launchable
`*.previous.nro` backup, and restores the old file if replacement fails. If you
launch that backup for recovery, B can skip updating for the current session.
Filesystem/power-loss and automatic restart behavior still need hardware testing.

**One manual upgrade is required to install this updater-enabled build.** Later
updates come from your server, never GitHub. The server must include the new
Switch-package endpoint; old servers continue serving Go handheld packages.

Server owners publish with `python manage.py publish_handheld PACKAGE --platform
switch`. Metadata/download use `/api/handheld/?platform=switch` and
`/api/handheld/download/?platform=switch`; default URLs remain the Go package.
Container builds pin both `handheld-version` and `switch-version`, bundle both
archives, and install them into the media volume at startup. GitHub release CI
now builds the NRO/ZIP as well as the existing Go archive.

## Controls

| Input | Action |
| --- | --- |
| D-pad / left analog stick | Navigate, with hold repeat |
| A | Open platform/playlist; toggle focused ROM; retry after pairing error |
| B | Back; cancel pairing; cancel active download queue; exit from home |
| Y | Select all / none in current view, including unloaded API pages |
| X (home) | Choose the ROM base directory |
| ZL (home) | Check/retry app updates from the paired server |
| L / R | Previous / next visible page (8 ROMs in the grid) |
| Plus | Download global selection |
| Minus | Exit (cancels active downloads) |

Bindings follow Go logical controls; Plus corresponds to Start. Selection is
shared across platforms and playlists and deduplicated by ROM ID. L/R scroll
within a lazily loaded grid rather than following pagination URLs. The Go
client has no search UI, so the Switch port does not add one.

## ROM directory picker

Press **X** on home to browse existing SD-card directories. The picker starts at
your current ROM base, or the nearest existing parent if the default folder has
not yet been created. D-pad/left stick navigates; A enters a directory; B goes to
the parent (or returns home at the SD root); Y uses the current directory; X
cancels. L/R scroll by a visible page. The saved base remains displayed while
browsing. Directory reads run on a worker, not the SDL rendering thread.

Only Y changes the saved base. Cancelling does not save anything. Pairing must
have completed before the picker can be used. Existing ROMs stay where they are;
the new base applies to future downloads and existing-file detection there.
Each platform retains its configured subfolder beneath that base. For example,
choosing `sdmc:/ROMs/` sends NES games to `sdmc:/ROMs/nes/` by default. The picker
selects existing folders; missing platform subfolders are created on download.

These are birdROMs defaults, not a required RetroArch directory layout.
RetroArch can browse/scan your chosen folder; no RetroArch settings or playlists
are changed automatically. Per-platform overrides remain in `platform_dirs`.

## Configuration

Path: `sdmc:/switch/birdroms/config.json` (on a computer:
`/switch/birdroms/config.json`). Created automatically only after pairing receives a device token. Entering a
server URL, waiting for approval, cancelling or failing pairing does not write
configuration files. A minimal optional first-run file:

```json
{
  "version": 1,
  "server_url": "https://roms.example.org",
  "device_token": "",
  "rom_base": "sdmc:/retroarch/roms/",
  "platform_dirs": {
    "Nintendo Entertainment System": "nes",
    "Super Nintendo Entertainment System": "snes",
    "Nintendo 64": "n64",
    "Custom Platform": "custom"
  }
}
```

`platform_dirs` keys must match the platform names returned by your server.
Built-in aliases are derived from the Go client's platform CSV, with RetroArch
folder overrides (`nes`, `snes`, `n64`, `gb`, `gbc`, `gba`, `genesis`, `psx`,
etc.). Unmapped platforms use `_UNSORTED`. Custom values must be single safe
folder names; subdirectories, separators and traversal are rejected. The ROM
base must start `sdmc:/`. Choose the base directory in-app with **X on the home screen**. Platform mapping
changes are made by editing this file while the application is closed. UI layout is fixed; there are no
additional UI settings to persist. Version 1 configuration accepts absent
optional fields, and saves with a temporary file and sync. On Switch, where rename cannot
replace an existing file, the app backs up the old config before moving the new
file into place. Failed replacements restore the previous file; interrupted
replacements recover a valid staged config or backup on the next launch.
The newly received token stays in memory while saving, and A retries a failed save.

The token is a secret. Do not share config.json. It is not printed in logs or
embedded in the binary. SD FAT/exFAT filesystems do not enforce Unix permissions.
To pair again, clear `device_token` while the app is closed. A server HTTP 401
clears the token in memory and starts pairing; the existing file is replaced
only after a new token arrives. Malformed configuration causes
a startup error; repair or back up and remove the file before relaunching.

## Network and downloads

- Device authentication and pairing match the existing Django API. No backend
  changes are required. See [AUDIT.md](AUDIT.md) for source references/contracts.
- HTTPS peer and hostname validation stay enabled. Use a server with a valid
  certificate and correct system time. Switch HTTPS uses the native system
  certificate store through libnx SSL. External cover hosts receive no device
  token. Redirects are rejected to prevent credential forwarding.
- API, pairing, cover requests and downloads execute on background workers.
  Rendering stays on the SDL thread. ROMs stream directly to SD; whole ROMs
  are never buffered in memory.
- Downloads are sequential. Progress shows percentage, bytes, speed, completed,
  skipped and failed counts. B requests cancellation. Files remain `.part`
  until the body and expected size are validated, flushed, synced and renamed.
- Existing files are skipped by default; known sizes must match. Tagged-name
  matches also require equal size and extension. These checks do not compute
  content hashes. Name collisions with different-sized files use an ID suffix;
  occupied destinations are preserved.
- Original filenames and extensions are preserved when filesystem-safe. Unsafe
  characters are replaced; names are bounded for FAT/exFAT; traversal and
  symlinks are rejected. Directories are created automatically. Archives are
  not extracted. RetroArch configuration/playlists are untouched.
- Backend download source does not implement HTTP Range responses. Cancelled
  or failed `.part` files are retained for inspection and **restarted** on retry.
  Completed files are never overwritten. Unknown-size files cannot receive a
  metadata size check, but HTTP transfer errors still prevent finalization.
- Requests have a 15-second connection timeout and a 30-second low-speed timeout;
  metadata/cover requests also have a 45-second overall timeout. Exiting can
  wait for an outstanding metadata/cover request to finish its bounded timeout.
- Cover data is capped at 4 MiB per image and checked before decoding (PNG/JPEG,
  up to 4096 pixels per dimension and 4 million total pixels). The texture cache
  retains 24 scaled images. Missing, unsupported or invalid images display placeholders.
  WebP-only covers currently use a placeholder.

## Validation and limitations

`tests/run.sh` runs API parsing/pairing/pagination tests, local real-HTTP fixture
tests for authentication/header safety/streaming/failure/cancellation, storage
and configuration tests (including traversal, symlinks, collisions, short reads
and `.part` restart), selection/navigation tests, and SD directory picker safety/navigation tests. `validate-nro.py` checks
NRO magic and embedded icon, metadata and RomFS asset ranges. These are build
and host checks, not proof of working hardware, live-server connectivity,
controller behavior or SD performance.

Switch automatic updates use a Switch-specific ZIP from the same server. The
Go muOS/ROCKNIX archive format, muOS catalogue box-art installation and game-list
refresh remain platform-specific. Cover art is shown in-app; RetroArch playlists/configuration
are not modified. No NSP/XCI, cores, saves, firmware or sysmodule changes.

## Physical Switch checklist

- [ ] Launch from Homebrew Menu in full-memory mode; verify title/icon and clean exit.
- [ ] Launch in applet mode and note memory/software-keyboard limitations.
- [ ] Enter server URL; approve QR/code on phone; restart and confirm token persistence.
- [ ] Pending, expired, rate-limited, denied/revoked pairing and offline errors are readable.
- [ ] Validate HTTPS against a public trusted server; reject invalid certificates/hostnames.
- [ ] Browse platforms and playlists; verify ordering, metadata, missing covers and large libraries.
- [ ] Joy-Con and Pro Controller: D-pad, analog repeat, A/B/Y/L/R/Plus/Minus.
- [ ] Select across views; Y includes all pages; navigate back during a request without stale results.
- [ ] X opens the directory picker; A/B navigate; X cancels without saving; Y persists a new base across restart.
- [ ] Changing the ROM base preserves pairing/platform mappings and routes future downloads to the new base.
- [ ] Download small and multi-gigabyte ROMs; UI stays responsive, names/extensions preserved.
- [ ] Verify ROMs open in RetroArch using the configured paths.
- [ ] Verify existing-file skip, filename collisions and custom/unknown platform mappings.
- [ ] Cancel, disconnect Wi-Fi and relaunch; no incomplete final files, retry restarts partials.
- [ ] Full/removed/write-protected SD errors preserve completed files and report failures.
- [ ] ZL checks the paired server; a newer Switch build downloads, verifies, installs and restarts/reopens.
- [ ] B skips an update; failure retains a readable error until explicit retry; offline checks do not block browsing.
- [ ] Existing writable config/token/ROM paths survive an update and can still be edited afterward.
- [ ] Launch `.previous.nro` for recovery and skip updating with B.
- [ ] Suspend/resume, dock/undock and exit during network/download operations; no hangs/leaks.

Third-party notices: [THIRD_PARTY.md](THIRD_PARTY.md).
