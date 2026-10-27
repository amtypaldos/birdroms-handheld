# birdROMs handheld

On-device client for [birdROMs](https://github.com/amtypaldos/birdROMs): Go + SDL2 app for muOS (RG40XX-V first).

## Build

Cross-compiled from macOS to a **dynamic arm64** binary linked against the device's own SDL2/SDL2_ttf (the device ships them; they need glibc >= 2.38, so the target is `aarch64-linux-gnu.2.38`).

Requirements: `go`, `zig` (C cross-compiler), `just`, `sshpass`.

One-time `sysroot/` (git-ignored), from the device and upstream:

- `sysroot/lib/`: `libSDL2-2.0.so.0.2800.5`, `libSDL2_ttf-2.0.so.0.2200.0`, `libfreetype.so.6.20.1` copied from the device's `/usr/lib` (plus `libSDL2.so` / `libSDL2_ttf.so` / `libfreetype.so.6` symlinks).
- `sysroot/include/SDL2/`: headers from SDL 2.28.5 and SDL_ttf 2.22.0 release tarballs.

```
just test      # pure-Go unit tests
just build     # dist/birdroms (arm64)
just deploy    # build + push to /mnt/mmc/MUOS/application/birdROMs on muos.local
```

Then launch **birdROMs** from muOS Applications. The app writes `birdroms.log` next to the binary.

## Device facts

See [docs/b1-device-recon.md](docs/b1-device-recon.md).
