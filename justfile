# birdROMs handheld — task runner
sysroot := justfile_directory() / "sysroot"

default:
    @just --list

test:
    go test ./internal/...

# Cross-compile the dynamic arm64 binary against the device's own SDL2 (needs zig + sysroot/).
build:
    mkdir -p dist
    PKG_CONFIG=true CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
    CC="zig cc -target aarch64-linux-gnu.2.38" \
    CGO_CFLAGS="-I{{sysroot}}/include -I{{sysroot}}/include/SDL2 -D_REENTRANT -DHAVE_STDLIB_H=1 -DHAVE_STRING_H=1" \
    CGO_LDFLAGS="-L{{sysroot}}/lib -lSDL2 -lSDL2_ttf -Wl,--allow-shlib-undefined" \
    go build -trimpath -ldflags "-s -w" -o dist/birdroms ./cmd/birdroms

# Push the binary + font to the device and run it (SSH password via SSHPASS, default root).
deploy: build
    SSHPASS=${SSHPASS:-root} sshpass -e ssh root@muos.local 'mkdir -p /mnt/mmc/MUOS/application/birdROMs/assets/fonts'
    SSHPASS=${SSHPASS:-root} sshpass -e scp dist/birdroms root@muos.local:/mnt/mmc/MUOS/application/birdROMs/birdroms
    SSHPASS=${SSHPASS:-root} sshpass -e scp assets/fonts/ChakraPetch-Bold.ttf root@muos.local:/mnt/mmc/MUOS/application/birdROMs/assets/fonts/
    SSHPASS=${SSHPASS:-root} sshpass -e scp package/mux_launch.sh package/mux_lang.ini root@muos.local:/mnt/mmc/MUOS/application/birdROMs/
    SSHPASS=${SSHPASS:-root} sshpass -e ssh root@muos.local 'chmod +x /mnt/mmc/MUOS/application/birdROMs/mux_launch.sh /mnt/mmc/MUOS/application/birdROMs/birdroms && sync'

# Build dist/birdROMs.muxapp (install via the muOS Archive Manager). Needs the same toolchain as `build`.
package: build
    #!/usr/bin/env bash
    set -euo pipefail
    stage="$(mktemp -d)"
    trap 'rm -rf "$stage"' EXIT
    app="$stage/application/birdROMs"
    mkdir -p "$app/assets/fonts"
    cp dist/birdroms package/mux_launch.sh package/mux_lang.ini "$app/"
    cp assets/fonts/ChakraPetch-Bold.ttf "$app/assets/fonts/"
    go run ./cmd/mkmuxapp "$stage" dist/birdROMs.muxapp
    echo "wrote dist/birdROMs.muxapp"

# Run natively on this machine (needs `brew install sdl2 sdl2_ttf pkgconf`); keys: arrows, Z/X=A/B, A/S=X/Y, Q/W=L1/R1, Enter=Start, Esc=Menu.
run:
    go run ./cmd/birdroms
