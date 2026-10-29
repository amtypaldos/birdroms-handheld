# Contributing to birdROMs handheld

Thanks for helping! This is the muOS client for [birdROMs](https://github.com/amtypaldos/birdROMs).
Please read the [Code of Conduct](CODE_OF_CONDUCT.md) first.

## Setup

You need Go (see `go.mod`), [just](https://github.com/casey/just), and SDL2 for the desktop build
(`brew install sdl2 sdl2_ttf pkgconf` on macOS, `apt install libsdl2-dev libsdl2-ttf-dev` on Debian/Ubuntu).
`just run` runs the app natively on your computer with the keyboard standing in for the handheld's
buttons, so most UI work needs no device. The README explains cross-building for the device.

## Before you open a pull request

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...     # or `just test` for the pure-Go packages
just build        # arm64 cross-build (needs zig and a sysroot; see the README)
```

CI runs the same checks, plus `just package`.

- **Tests with your change.** New behaviour and bug fixes come with tests. The UI packages are
  thin; put logic in `internal/` where it can be tested without SDL.
- **Keep it small.** One logical change per PR, with a description of what and why.
- **ROM file names are sacred.** muOS and RetroArch key saves, states and box art to the ROM's
  file name, so downloads must keep the server's original file name.
- **Read-only client.** The device only ever sends `GET` requests to the server.
- **No ROMs, no secrets.** Never commit ROMs, BIOS files, copyrighted game art, tokens or
  `config.json`. Use synthetic data in tests.

By contributing you agree that your contributions are licensed under the project's [MIT License](LICENSE).
