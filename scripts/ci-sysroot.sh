#!/usr/bin/env bash
# Build a link-time sysroot for the handheld binary WITHOUT a device.
#
#   scripts/ci-sysroot.sh [dest]        (default dest: ./sysroot)
#
# The binary is a dynamic arm64 cgo program that links against SDL2 / SDL2_ttf and uses the
# copies shipped on the device at run time. To compile and link we only need
#   * SDL2 and SDL2_ttf headers  -> the official release tarballs, pinned to the versions muOS
#     ships (SDL 2.28.5, SDL_ttf 2.22.0) and verified by SHA-256, so the binary can never
#     reference an API the device lacks;
#   * libSDL2 / libSDL2_ttf / libfreetype shared objects to link against -> the current Ubuntu
#     24.04 (noble) arm64 packages, resolved through the package index (fetched over HTTPS) and
#     verified by the SHA-256 listed there. They are link stubs only: SDL has no symbol
#     versioning and the soname matches the device's (libSDL2-2.0.so.0), so at run time the
#     device's own libraries are used.
# The glibc the binary needs is decided by zig's target (aarch64-linux-gnu.2.38), not by these
# libraries; `scripts/check-glibc.sh dist/birdroms` verifies that.
#
# Needs: curl, tar, gzip, sha256sum or shasum, and dpkg-deb or ar+zstd/xz.
set -euo pipefail

DEST="${1:-sysroot}"
SDL_VERSION=2.28.5
SDL_SHA256=332cb37d0be20cb9541739c61f79bae5a477427d79ae85e352089afdaf6666e4
TTF_VERSION=2.22.0
TTF_SHA256=d48cbd1ce475b9e178206bf3b72d56b66d84d44f64ac05803328396234d67723
MIRROR="${UBUNTU_PORTS_MIRROR:-https://ports.ubuntu.com/ubuntu-ports}"
SERIES="${UBUNTU_SERIES:-noble}"
PACKAGES=(libsdl2-2.0-0 libsdl2-ttf-2.0-0 libfreetype6)

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
verify() { [ "$(sha256 "$1")" = "$2" ] || { echo "ci-sysroot: SHA-256 mismatch for $1" >&2; exit 1; }; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$DEST/include" "$DEST/lib"
DEST="$(cd "$DEST" && pwd)"

# --- headers ---------------------------------------------------------------------------------
fetch_headers() { # name version sha url-prefix
  local name="$1" ver="$2" sha="$3" base="$4"
  curl -fsSL -o "$work/$name.tar.gz" "$base/release-$ver/$name-$ver.tar.gz"
  verify "$work/$name.tar.gz" "$sha"
  tar -xzf "$work/$name.tar.gz" -C "$work"
}
fetch_headers SDL2 "$SDL_VERSION" "$SDL_SHA256" https://github.com/libsdl-org/SDL/releases/download
fetch_headers SDL2_ttf "$TTF_VERSION" "$TTF_SHA256" https://github.com/libsdl-org/SDL_ttf/releases/download
mkdir -p "$DEST/include/SDL2"
cp "$work/SDL2-$SDL_VERSION"/include/*.h "$DEST/include/SDL2/"
cp "$work/SDL2_ttf-$TTF_VERSION/SDL_ttf.h" "$DEST/include/SDL2/"

# --- shared objects -------------------------------------------------------------------------
for suite in "$SERIES" "$SERIES-updates"; do
  for comp in main universe; do
    curl -fsSL "$MIRROR/dists/$suite/$comp/binary-arm64/Packages.gz" | gzip -dc >"$work/Packages-$suite-$comp"
  done
done

# Print "<filename> <sha256>" for the newest suite that has the package.
lookup() {
  local pkg="$1" f
  for f in "$work"/Packages-"$SERIES"-updates-main "$work"/Packages-"$SERIES"-updates-universe \
           "$work"/Packages-"$SERIES"-main "$work"/Packages-"$SERIES"-universe; do
    [ -f "$f" ] || continue
    awk -v p="$pkg" '
      /^Package: / { hit = ($2 == p) }
      hit && /^Filename: / { fn = $2 }
      hit && /^SHA256: /   { print fn, $2; exit }' "$f" | grep . && return 0
  done
  return 1
}

extract_deb() { # deb dir
  if command -v dpkg-deb >/dev/null; then dpkg-deb -x "$1" "$2"; return; fi
  local member
  member="$(ar t "$1" | grep '^data.tar')"
  case "$member" in
    *.zst) ar p "$1" "$member" | zstd -dc | tar -x -C "$2" ;;
    *.xz)  ar p "$1" "$member" | xz -dc | tar -x -C "$2" ;;
    *)     ar p "$1" "$member" | tar -x -C "$2" ;;
  esac
}

for pkg in "${PACKAGES[@]}"; do
  read -r filename sum < <(lookup "$pkg") || { echo "ci-sysroot: $pkg not found in $SERIES" >&2; exit 1; }
  echo "ci-sysroot: $pkg <- $filename"
  curl -fsSL -o "$work/$pkg.deb" "$MIRROR/$filename"
  verify "$work/$pkg.deb" "$sum"
  mkdir -p "$work/x-$pkg"
  extract_deb "$work/$pkg.deb" "$work/x-$pkg"
  # -a keeps the soname symlinks; the .deb ships them under usr/lib/aarch64-linux-gnu.
  cp -a "$work/x-$pkg"/usr/lib/aarch64-linux-gnu/*.so* "$DEST/lib/"
done

# Unversioned names for -lSDL2 / -lSDL2_ttf.
(
  cd "$DEST/lib"
  ln -sf "$(ls libSDL2-2.0.so.0.* | head -1)" libSDL2.so
  ln -sf "$(ls libSDL2_ttf-2.0.so.0.* | head -1)" libSDL2_ttf.so
)
echo "ci-sysroot: wrote $DEST (SDL $SDL_VERSION / SDL_ttf $TTF_VERSION headers)"
