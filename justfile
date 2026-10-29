# birdROMs handheld — task runner
# Link-time SDL2 headers/libs; `scripts/ci-sysroot.sh` creates one without a device. Override with SYSROOT=...
sysroot := env("SYSROOT", justfile_directory() / "sysroot")
# Where `just deploy` pushes the app (muOS default hostname; set BIRDROMS_DEVICE=<host or ip> to override).
device := env("BIRDROMS_DEVICE", "muos.local")
# Version stamped into the binary (releases set VERSION from the tag).
version := env("VERSION", "dev")

default:
    @just --list

test:
    go test ./internal/...

# Every package, including the SDL/cgo UI ones (needs SDL2 headers: `brew install sdl2 sdl2_ttf pkgconf`).
test-all:
    go test ./...

# Cross-compile the dynamic arm64 binary against the device's own SDL2 (needs zig + sysroot/).
build:
    mkdir -p dist
    PKG_CONFIG=true CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
    CC="zig cc -target aarch64-linux-gnu.2.38" \
    CGO_CFLAGS="-I{{sysroot}}/include -I{{sysroot}}/include/SDL2 -D_REENTRANT -DHAVE_STDLIB_H=1 -DHAVE_STRING_H=1" \
    CGO_LDFLAGS="-L{{sysroot}}/lib -lSDL2 -lSDL2_ttf -Wl,--allow-shlib-undefined" \
    go build -trimpath -ldflags "-s -w -X github.com/amtypaldos/birdroms-handheld/internal/version.Version={{version}}" -o dist/birdroms ./cmd/birdroms

# Push the binary + font into the app folder muOS actually launches from (/run/muos/storage/application,
# which follows the configured app storage, e.g. the SD card); SSH password via SSHPASS, default root.
deploy: build
    SSHPASS=${SSHPASS:-root} sshpass -e ssh root@{{device}} 'mkdir -p /run/muos/storage/application/birdROMs/assets/fonts /run/muos/storage/application/birdROMs/glyph'
    SSHPASS=${SSHPASS:-root} sshpass -e scp dist/birdroms root@{{device}}:/run/muos/storage/application/birdROMs/birdroms
    SSHPASS=${SSHPASS:-root} sshpass -e scp assets/fonts/ChakraPetch-Bold.ttf root@{{device}}:/run/muos/storage/application/birdROMs/assets/fonts/
    SSHPASS=${SSHPASS:-root} sshpass -e scp package/glyph/birdroms.svg root@{{device}}:/run/muos/storage/application/birdROMs/glyph/
    SSHPASS=${SSHPASS:-root} sshpass -e scp package/mux_launch.sh package/mux_lang.ini root@{{device}}:/run/muos/storage/application/birdROMs/
    SSHPASS=${SSHPASS:-root} sshpass -e ssh root@{{device}} 'chmod +x /run/muos/storage/application/birdROMs/mux_launch.sh /run/muos/storage/application/birdROMs/birdroms && sync'

# Build dist/birdROMs.muxapp (install via the muOS Archive Manager). Needs the same toolchain as `build`.
package: build
    #!/usr/bin/env bash
    set -euo pipefail
    stage="$(mktemp -d)"
    trap 'rm -rf "$stage"' EXIT
    app="$stage/birdROMs"
    mkdir -p "$app/assets/fonts" "$app/glyph"
    cp dist/birdroms package/mux_launch.sh package/mux_lang.ini "$app/"
    cp assets/fonts/ChakraPetch-Bold.ttf assets/fonts/OFL.txt "$app/assets/fonts/"
    cp LICENSE THIRD_PARTY_NOTICES.txt "$app/"
    cp package/glyph/birdroms.svg "$app/glyph/"
    go run ./cmd/mkmuxapp "$stage" dist/birdROMs.muxapp
    echo "wrote dist/birdROMs.muxapp"

# Run natively on this machine (needs `brew install sdl2 sdl2_ttf pkgconf`); keys: arrows, Z/X=A/B, A/S=X/Y, Q/W=L1/R1, Enter=Start, Esc=Menu.
run:
    go run ./cmd/birdroms
