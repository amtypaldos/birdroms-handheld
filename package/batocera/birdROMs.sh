#!/bin/bash
# birdROMs Batocera alpha. Extract this script and birdroms/ into the Ports folder.
# The firmware's Ports runner owns frontend/display lifecycle; do not stop it.

fail() {
    printf 'birdROMs: %s\n' "$*" >&2
    exit 1
}

BIRDROMS_PORTS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || exit 1
BIRDROMS_APP_DIR="$BIRDROMS_PORTS_DIR/birdroms"
BIRDROMS_APP_BIN="$BIRDROMS_APP_DIR/birdroms"
[ -f "$BIRDROMS_APP_BIN" ] || fail "Missing binary: $BIRDROMS_APP_BIN. Extract the complete Batocera ZIP."

case "$(uname -m)" in
    aarch64|arm64) ;;
    *) fail "This package needs aarch64 Batocera (64-bit ARM userspace)." ;;
esac

export BIRDROMS_FIRMWARE=batocera
export BIRDROMS_ROM_ROOT="${BIRDROMS_ROM_ROOT:-$(dirname -- "$BIRDROMS_PORTS_DIR")}"
[ -d "$BIRDROMS_ROM_ROOT" ] || fail "ROM storage is missing: $BIRDROMS_ROM_ROOT. No directories were created."
case "$BIRDROMS_ROM_ROOT" in
    /*) ;;
    *) fail "ROM storage must be an absolute path." ;;
esac
[ "$BIRDROMS_ROM_ROOT" != / ] || fail "ROM storage cannot be the system root."

# Batocera's normal Ports generator exports inline SDL mappings and writes
# /tmp/gamecontrollerdb.txt from the configured EmulationStation controllers.
# Preserve its display/render/library environment. Do not source PortMaster or
# another firmware's helpers: they would replace this mapping and environment.
valid_controller_file() {
    [ -r "$1" ] && LC_ALL=C grep -qE '^[[:space:]]*[[:xdigit:]]{32},[^,]+,' "$1"
}
valid_controller_text() {
    printf '%s\n' "$1" | LC_ALL=C grep -qE '^[[:space:]]*[[:xdigit:]]{32},[^,]+,'
}

if [ -n "${SDL_GAMECONTROLLERCONFIG_FILE:-}" ] && ! valid_controller_file "$SDL_GAMECONTROLLERCONFIG_FILE"; then
    unset SDL_GAMECONTROLLERCONFIG_FILE
    printf 'birdROMs: supplied controller database is unavailable; checking the Batocera-generated database.\n' >&2
fi
if [ -z "${SDL_GAMECONTROLLERCONFIG_FILE:-}" ] && valid_controller_file /tmp/gamecontrollerdb.txt; then
    export SDL_GAMECONTROLLERCONFIG_FILE=/tmp/gamecontrollerdb.txt
fi
if [ -n "${SDL_GAMECONTROLLERCONFIG:-}" ] && ! valid_controller_text "$SDL_GAMECONTROLLERCONFIG"; then
    unset SDL_GAMECONTROLLERCONFIG
    printf 'birdROMs: supplied inline controller mapping is invalid.\n' >&2
fi
if [ -z "${SDL_GAMECONTROLLERCONFIG:-}" ] && [ -z "${SDL_GAMECONTROLLERCONFIG_FILE:-}" ]; then
    printf 'birdROMs: no configured controller mapping. Configure a controller in EmulationStation and launch birdROMs through Ports.\n' >&2
fi

if [ ! -x "$BIRDROMS_APP_BIN" ]; then
    chmod +x "$BIRDROMS_APP_BIN" 2>/dev/null || true
    [ -x "$BIRDROMS_APP_BIN" ] || fail "The binary is not executable. Check SD filesystem/mount options."
fi

# ldd is advisory when unavailable or when an executable is static; a missing
# shared library is a definite startup failure worth reporting before launch.
if command -v ldd >/dev/null 2>&1; then
    BIRDROMS_LIBRARIES="$(ldd "$BIRDROMS_APP_BIN" 2>&1)"
    if [[ "$BIRDROMS_LIBRARIES" == *"not found"* ]]; then
        printf '%s\n' "$BIRDROMS_LIBRARIES" >&2
        fail "A required runtime library is missing; use this ZIP on Batocera."
    fi
fi

cd -- "$BIRDROMS_APP_DIR" || fail "Could not enter the app directory."
# No binary/config replacement in this alpha. Returning the real process status
# lets the firmware's normal Ports runner restore the frontend after any exit.
./birdroms > birdroms.log 2>&1
BIRDROMS_STATUS=$?
if [ "$BIRDROMS_STATUS" -ne 0 ]; then
    printf 'birdROMs exited with status %s. See %s/birdroms.log\n' \
        "$BIRDROMS_STATUS" "$BIRDROMS_APP_DIR" >&2
fi
exit "$BIRDROMS_STATUS"
