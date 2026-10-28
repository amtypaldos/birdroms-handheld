# birdROMs handheld

On-device client for [birdROMs](https://github.com/amtypaldos/birdROMs): Go + SDL2 app for muOS (RG40XX-V first).

**Installing on a handheld?** Follow [docs/install.md](docs/install.md). Publishing a new version for friends: [docs/publishing.md](docs/publishing.md).

## Build

Cross-compiled from macOS to a **dynamic arm64** binary linked against the device's own SDL2/SDL2_ttf (the device ships them; they need glibc >= 2.38, so the target is `aarch64-linux-gnu.2.38`).

Requirements: `go`, `zig` (C cross-compiler), `just`; `sshpass` only for `just deploy`.

The build needs a git-ignored `sysroot/` holding the SDL2 / SDL2_ttf headers and shared objects to link against. Pick one:

- **No device needed (what CI uses):** `scripts/ci-sysroot.sh` downloads the SDL 2.28.5 / SDL_ttf 2.22.0 headers (the versions muOS ships, pinned by SHA-256) and Ubuntu 24.04 arm64 `libSDL2` / `libSDL2_ttf` / `libfreetype` as link stubs. At run time the device's own libraries are used. `scripts/check-glibc.sh dist/birdroms` checks that the result needs no glibc newer than 2.38.
- **From your device:** copy `libSDL2-2.0.so.*`, `libSDL2_ttf-2.0.so.*` and `libfreetype.so.6*` from the device's `/usr/lib` into `sysroot/lib/` (plus `libSDL2.so` / `libSDL2_ttf.so` symlinks), and the SDL 2.28.5 / SDL_ttf 2.22.0 headers from their release tarballs into `sysroot/include/SDL2/`.

Use another location with `SYSROOT=/path just build`.

```
just test      # pure-Go unit tests (`just test-all` also covers the SDL UI packages)
just build     # dist/birdroms (arm64)
just package   # dist/birdROMs.muxapp (entries rooted at birdROMs/)
just run       # run natively on your computer (needs SDL2: brew install sdl2 sdl2_ttf pkgconf)
just deploy    # dev only: build + push into the app folder on a device over SSH (BIRDROMS_DEVICE=<host>, default muos.local)
```

Then launch **birdROMs** from muOS Applications. The app writes `birdroms.log` next to the binary.

## Box art

After a ROM downloads, its cover is saved where muOS shows box art: `<catalogue>/<catalogue name>/box/<ROM file name without extension>.png` (named after the file on the card, so it matches the saved original filename). The catalogue is `/run/muos/storage/info/catalogue` (else `<ROM storage>/MUOS/info/catalogue`). The catalogue name is read from muOS's own assignment (`/opt/muos/share/info/content/<FOLDER>/core.cfg` line 3, or a per-ROM cfg line 4); a folder with no assignment falls back to muOS's `info/manifest/assign.json` table, and only if that catalogue folder already exists. Existing art is never replaced, art is written atomically, covers over 600 px are scaled down, and any failure is only logged. Folders muOS has no catalogue for (e.g. `_UNSORTED`) get no art. Override paths for testing with `BIRDROMS_CATALOGUE_DIR` / `BIRDROMS_CONTENT_DIR`.

## Device facts

See [docs/b1-device-recon.md](docs/b1-device-recon.md).

## Install

Friends install the app from a `birdROMs.muxapp` package through the muOS Archive Manager: copy it into an `ARCHIVE/` folder on the card, open **Archive Manager**, pick it, then launch **birdROMs** from Applications. Packages come from this repository's GitHub Releases, or from the **Handheld** page of a birdROMs server (which also tells the app where the server is). Step-by-step instructions: [docs/install.md](docs/install.md).

## Using it

1. **Pair.** On first launch the app shows a QR code and a short code. Scan the QR code with your phone, sign in to the birdROMs server and approve the device. The server address comes from the `BIRDROMS_SERVER` environment variable or `server_url` in `config.json` next to the binary. The resulting device token is stored in `config.json` (mode 0600); it is read-only and the server owner can revoke it.
2. **Browse** playlists and platforms (cover-art grid, one square tile per ROM).
3. **Select and download.** Selected ROMs download one at a time into `Roms/<SYSTEM>` on the card, using the original file name so saves and states keep working. Platforms muOS has no folder for go to `Roms/_UNSORTED`.

| Button | Action |
| --- | --- |
| D-pad | Move |
| A | Open / select |
| Y | Select all / none in the current grid |
| Start | Download the selection |
| B | Back / exit |
| Menu (the muOS guide button) | Leave the app |

On a desktop (`just run`) the keys are: arrows, `Z` = A, `X` = B, `A` = X, `S` = Y, `Q`/`W` = L1/R1, `Enter` = Start, `Esc` = Menu.

`mux_launch.sh` sets `BIRDROMS_ROM_MOUNT` (where ROMs are written) and `BIRDROMS_ROM_MOUNTS` (every storage, used to skip ROMs you already have). The app writes `birdroms.log` next to the binary.

## Releases and CI

- **CI** (`.github/workflows/ci.yml`) runs `gofmt`, `go vet` and `go test ./...`, then cross-builds the arm64 binary with zig against the CI sysroot, checks it needs no glibc newer than 2.38, builds the `.muxapp` and uploads it as a workflow artifact.
- **Releases:** pushing a date tag like `v2026.10.28` (`v2026.10.28.1` for a second release that day) runs `.github/workflows/release.yml`, which attaches `birdROMs.muxapp` (and its SHA-256) to a GitHub Release.

## Legal

birdROMs does not ship or distribute any ROMs, BIOS files or other copyrighted game content, and this app only downloads what a birdROMs server's owner has chosen to share with you. You are responsible for only storing and sharing content you have the right to use.

## License

MIT, see [LICENSE](LICENSE). The bundled Chakra Petch font is under the SIL Open Font License 1.1 ([assets/fonts/OFL.txt](assets/fonts/OFL.txt)); other third-party notices are in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt) and ship inside the `.muxapp`. See also [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).
