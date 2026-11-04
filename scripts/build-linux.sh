#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
target="${1:?usage: build-linux.sh TARGET VERSION (rocknix or batocera)}"
version="${2:?version required}"
case "$target" in
 rocknix) sysroot="${ROCKNIX_SYSROOT:-$PWD/sysroot/rocknix}" ;;
 batocera) sysroot="${BATOCERA_SYSROOT:-$PWD/sysroot/batocera}" ;;
 *) echo "Unsupported Linux target: $target" >&2; exit 1 ;;
esac
[ -f "$sysroot/include/SDL2/SDL_ttf.h" ] || python3 scripts/linux-sysroot.py "$sysroot"
mkdir -p "dist/$target"
PKG_CONFIG=true CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
CC="zig cc -target aarch64-linux-gnu.2.17" \
CGO_CFLAGS="-I$sysroot/include -I$sysroot/include/SDL2 -D_REENTRANT -DHAVE_STDLIB_H=1 -DHAVE_STRING_H=1" \
CGO_LDFLAGS="-L$sysroot/lib -lSDL2 -lSDL2_ttf -Wl,--allow-shlib-undefined" \
go build -trimpath -ldflags "-s -w -X github.com/amtypaldos/birdroms-handheld/internal/version.Version=$version" -o "dist/$target/birdroms" ./cmd/birdroms
scripts/check-glibc.sh "dist/$target/birdroms" 2.17
readelf -h "dist/$target/birdroms" | grep -q 'Machine:.*AArch64'
readelf -l "dist/$target/birdroms" | grep -q '/lib/ld-linux-aarch64.so.1'
