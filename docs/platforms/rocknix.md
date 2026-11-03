# ROCKNIX alpha

Native Go/SDL2 birdROMs client for **AArch64 ROCKNIX**. This is a hardware-pending
alpha: pairing, browsing, selection and downloads share the existing muOS client,
with ROCKNIX-specific paths, controller setup and packaging. No device runtime
success is claimed until the SD-card test is recorded.

## Install

1. Download `birdROMs-rocknix-aarch64.zip` from the ROCKNIX alpha GitHub release.
2. Extract its contents **into your ROM card's `roms/ports` directory**. Keep
   `birdROMs.sh` beside the complete `birdroms` directory:

   ```text
   roms/ports/
   ├── birdROMs.sh
   └── birdroms/
       ├── birdroms
       ├── assets/fonts/
       ├── version.json
       └── server.json
   ```

3. Confirm `birdroms/server.json` contains your birdROMs URL. The personal-test
   alpha release is seeded with `https://retro.birdperson.net`. For another
   server, edit only this file before first pairing:

   ```json
   {"server_url":"https://your-birdroms-server.example"}
   ```

4. Refresh the ROCKNIX game list if needed, open **Ports → birdROMs**, and approve
   the displayed pairing code in your browser. A normal authenticated device
   token is saved only after pairing, in `birdroms/config.json`.

Run from Ports rather than SSH for normal display/controller setup. PortMaster's
existing controller helper is used when installed; otherwise the firmware
controller database is used. Opening PortMaster once can initialize its mappings.
Do not install a `.muxapp` or Switch `.nro` on ROCKNIX.

## Controls and storage

D-pad navigates; A opens/selects; B returns/exits; Y selects all/none; L1/R1 page;
Start downloads selected ROMs. Guide exits. Keyboard controls remain those of
the existing Go client. Analog-stick navigation is not part of this alpha.

ROMs go into the root containing `ports`, using native system directories such
as `nes`, `snes`, `gb`, `gbc`, `gba`, `genesis` and `psx`. The launcher resolves
its actual card location, including paths containing spaces. Original ROM names
and extensions are kept. Existing files are checked before download. Unknown
systems go into `_UNSORTED` with a warning; folder mapping does not imply that
the device can emulate that system.

Advanced launcher environment overrides:

- `BIRDROMS_ROM_ROOT`: existing absolute ROM directory, not a card parent.
- `BIRDROMS_ROM_ROOTS`: colon-separated complete ROM directories to check for
  existing files on other storage.
- `BIRDROMS_PLATFORM_DIRS`: JSON object of canonical platform-to-directory
  overrides, for example `{"Sega Genesis":"megadrive"}`. Unsafe paths are rejected.
- `BIRDROMS_SERVER`: optional server URL override before pairing.

The alpha does not modify emulator settings, gamelist XML, playlists or saves.
In-app covers work as in the shared client; external frontend scraping/art is
not implemented. Use ROCKNIX's normal game-list refresh to discover downloads.

## Updates and recovery

Automatic ROCKNIX updates and self-hosted ZIP distribution are **not enabled in
this alpha**. It cannot fetch or install a muOS update package. Install newer
alpha ZIPs manually, preserving `birdroms/config.json`; the ZIP never includes
that file or a device token. After pairing, the saved server URL takes priority
over `server.json`, so changing a seed does not redirect an existing pairing.

For startup failures, check `birdroms/birdroms.log` and ROCKNIX's Ports execution
log. Missing libraries, incompatible architecture and missing storage produce
launcher errors. Include the app version, exact model, ROCKNIX version, screen
behavior and redacted log when reporting results. Never share config.json or a
device token. Back up an existing ROM card before changing firmware.

## Build and provenance

Requires Go from `go.mod`, Zig **0.17.0**, Python 3, dpkg-deb, readelf and normal
Linux build tools. Host tests additionally need SDL2/SDL2_ttf development packages.

```bash
scripts/rocknix-sysroot.py sysroot/rocknix
scripts/build-linux.sh rocknix 2026.11.03-alpha.1
scripts/package-linux.sh rocknix 2026.11.03-alpha.1 \
  --server-url https://retro.birdperson.net
```

Output: `dist/birdROMs-rocknix-aarch64.zip` and its `.sha256`. No runtime SDL
libraries are bundled. The build targets glibc **2.17**, SDL2 headers **2.28.5**
and SDL2_ttf headers **2.20.2**. Link-only Ubuntu ARM64 shared objects and official
header archives are SHA-256 pinned. The glibc ceiling and AArch64 ELF/interpreter
are checked after building; actual firmware compatibility still needs hardware.

Source audit is based on ROCKNIX distribution commit
[`93c889bb853f2a11e51cecb0a9a75221219a2cce`](https://github.com/ROCKNIX/distribution/tree/93c889bb853f2a11e51cecb0a9a75221219a2cce).
Its SDL2 is 2.32.10 and SDL2_ttf 2.20.2; these are upstream source facts, not a
claim about your installed release. Controller helper and Ports lifecycle were
read from `portmaster/sources/control.txt` and `runemu.sh`. Folder mappings were
checked against [official ROCKNIX systems documentation](https://rocknix.org/systems/).

## Physical test checklist

- [ ] Record Anbernic model, ROCKNIX release, userspace architecture and filesystem.
- [ ] Launch from Ports; verify scaling, rotation and readable UI.
- [ ] Pair successfully; relaunch without pairing again; confirm config is writable.
- [ ] Browse platforms/playlists, covers and pages; exercise all documented buttons.
- [ ] Download representative ROMs; verify exact filenames, native folders and skip behavior.
- [ ] Cancel a download, retry, and test offline/error handling.
- [ ] Return to the frontend after normal exit and repeat launch/exit.
- [ ] Refresh game list and verify downloaded titles are visible.
- [ ] Manually replace the alpha while preserving config and verify pairing persists.

## Automated validation

The full Go/SDL suite and `go vet ./...` passed on Linux. The dummy-video SDL
test verifies a 640x480 logical viewport on a 1280x720 renderer. Shoulder paging
has a controller regression test. Firmware tests cover native folder mappings,
strict overrides, alternate roots and storage safety. Twelve launcher fixtures
cover normal/cancel/error exit, repeated launches, spaces, missing dependencies,
config preservation and broken controller-helper fallback. Four package CLI tests
cover archive identity/layout, reproducibility, wrong architecture and invalid
version/seed rejection. The actual cross-built ELF passed AArch64/interpreter
and GLIBC_2.17 checks; its packaged manifest matches binary bytes and hash.
These are build/host checks; the physical checklist above remains open.
