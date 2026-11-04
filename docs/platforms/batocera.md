# Batocera AArch64 alpha

Hardware-pending port of the shared Go/SDL2 client for **64-bit ARM Batocera**.
It uses Batocera's native Ports runner and controller mappings. It is a separate
package from ROCKNIX and KNULLI; it is not an x86_64 PC build.

## Installation

1. Confirm your exact device has a supported Batocera image and an AArch64
   userspace. Do not flash a firmware image intended for another model. KNULLI
   and older unofficial Anbernic Batocera images are separate compatibility
   targets and have not been validated by this package.
2. Extract `birdROMs-batocera-aarch64.zip` into `/userdata/roms/ports/` (the
   `roms/ports` directory inside your Batocera share). Keep this layout:

   ```text
   /userdata/roms/ports/
   ├── birdROMs.sh
   └── birdroms/
       ├── birdroms
       ├── assets/fonts/
       ├── version.json
       └── server.json
   ```

3. The test release's `server.json` points to `https://retro.birdperson.net`.
   If using another server, edit the URL before first pairing:

   ```json
   {"server_url":"https://your-server.example"}
   ```

4. Use **Main Menu → Game Settings → Update Games Lists** if the entry is not
   visible, then open **Ports → birdROMs**. Approve the pairing code in your
   server's web interface. Configuration and the device token are saved to
   `birdroms/config.json` after pairing succeeds.

Configure your gamepad in Batocera before launching. Start from Ports for the
proper display/controller environment; an SSH shell does not reproduce it.
This application does not require PortMaster and never sources its helpers.

## Controls and ROM folders

D-pad navigates, A opens/selects, B returns/exits, Y selects all/none, L1/R1 page,
Start downloads selected ROMs, and Guide exits. The shared UI uses a 640x480
logical layout scaled with preserved aspect ratio. Analog-stick navigation is
not implemented in this alpha.

Downloads use native system folders beneath the ROM root containing Ports,
usually `/userdata/roms`. Examples: `nes`, `snes`, `gb`, `gbc`, `gba`,
`megadrive`, `megacd`, `psx`, `neogeocd`. These mappings are independent of
ROCKNIX and muOS. Original filenames/extensions are preserved; existing files
are skipped. Unmapped platforms use `_UNSORTED` with a warning. A folder map
does not imply a device can emulate every mapped system.

Supported launcher environment overrides:

- `BIRDROMS_ROM_ROOT`: existing absolute ROM directory (not `/userdata` itself).
- `BIRDROMS_ROM_ROOTS`: colon-separated complete ROM roots for existing-file checks.
- `BIRDROMS_PLATFORM_DIRS`: strict JSON map, for example
  `{"Sega Genesis":"megadrive"}`; unsafe paths are rejected.
- `BIRDROMS_SERVER`: optional first-pairing server override.

The launcher resolves its installation directory, including spaces and alternate
storage paths, and refuses missing storage. Existing Batocera SDL environment
and controller configuration are preserved. No service restarts, firmware writes,
RetroArch settings, gamelist XML, emulator playlists or save changes are required.

In-app cover art is supported. Frontend scraping/art export is not implemented;
refresh game lists using Batocera's UI after downloading.

## Updates and troubleshooting

This alpha uses manual ZIP updates. Automatic Batocera updates and self-hosted
Batocera ZIP distribution remain future work. The muOS updater cannot run under
the Batocera profile. New ZIPs exclude `config.json`; preserve that file when
upgrading. An existing paired server URL takes priority over the URL-only seed.

Startup errors are reported by the launcher; native app logs are in
`birdroms/birdroms.log`. Missing runtime libraries, wrong architecture and absent
storage are diagnosed before launch. If controls do not respond, reconfigure
them in Batocera and relaunch from Ports. The launcher preserves the runner's
inline SDL mapping and uses its generated controller DB when valid. Do not share
config.json or authentication tokens when reporting a failure.

## Build and source audit

Requires Go from `go.mod`, Zig **0.17.0**, Python 3, dpkg-deb and readelf; host tests
also need SDL2/SDL2_ttf development packages.

```bash
python3 scripts/linux-sysroot.py sysroot/batocera
scripts/build-linux.sh batocera 2026.11.04-alpha.1
scripts/package-linux.sh batocera 2026.11.04-alpha.1 \
  --server-url https://retro.birdperson.net
```

Output is `dist/birdROMs-batocera-aarch64.zip` plus SHA-256. The shared pinned
AArch64 link SDK uses SDL2 2.28.5 and SDL2_ttf 2.20.2 headers and glibc 2.17.
Runtime libraries are provided by Batocera, not bundled. The build checks ELF
architecture/interpreter and the glibc ceiling. Sharing this SDK is based on
actual source ABI requirements rather than CPU architecture alone.

Audit target: [Batocera43.1 source `155c2d8d`](https://github.com/batocera-linux/batocera.linux/tree/155c2d8d304cbb53db52e9479dcf683392821d5c).
Folder names come from `batocera-es-system/es_systems.yml`; Ports/controller
behavior comes from `shGenerator.py`, `controller.py` and `emulatorlauncher.py`.
Its pinned Buildroot commit `2ef23189ecd626c334e071230e5cbe102b8c41f5`
uses SDL2 **2.32.8** and SDL2_ttf **2.24.0**, newer than this SDK's header baseline.
Those are source facts, not a runtime claim about a particular installed image.
See [Batocera Ports](https://wiki.batocera.org/systems:ports).

## Physical acceptance checklist

- [ ] Record exact device, Batocera version, userspace architecture, filesystem and mount layout.
- [ ] Launch from Ports; verify display, rotation, scaling and controller mappings.
- [ ] Pair, exit/relaunch, preserve token and confirm config remains writable.
- [ ] Browse playlists/platforms/covers, page, select and download with physical controls.
- [ ] Verify original filenames and native ROM folders, unknown warning and existing-file skips.
- [ ] Exercise cancellation, offline/network failure and storage-write errors.
- [ ] Exit normally and repeat launch/exit without losing frontend control.
- [ ] Refresh game lists and confirm downloads appear.
- [ ] Install a newer ZIP while preserving config and verify pairing persists.

## Automated validation

Full Go/SDL tests and vet passed, including the logical viewport test for both
ROCKNIX and Batocera. Firmware tests cover91 canonical mappings, target-specific
folder differences, safe overrides and an actual download preserving its
original filename beneath `megadrive` without adding `Roms`. Thirteen Batocera
and twelve ROCKNIX launcher fixtures passed; five packager CLI tests passed.
Shell syntax and Actionlint passed. The actual built AArch64 ELF requires only
GLIBC_2.17 symbol versions, and its ZIP manifest/hash/URL-only seed were verified.
Independent review found no important issues. These are automated/source checks;
physical runtime and SD-card behavior remain unverified.
