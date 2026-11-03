#!/bin/bash
# birdROMs ROCKNIX alpha. Extract this script and birdroms/ into the Ports folder.
# The firmware's Ports runner owns frontend/display lifecycle; do not stop it.

fail() {
    printf 'birdROMs: %s\n' "$*" >&2
    exit 1
}

BIRDROMS_PORTS_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)" || exit 1
BIRDROMS_APP_DIR="$BIRDROMS_PORTS_DIR/birdroms"
BIRDROMS_APP_BIN="$BIRDROMS_APP_DIR/birdroms"
[ -f "$BIRDROMS_APP_BIN" ] || fail "Missing binary: $BIRDROMS_APP_BIN. Extract the complete ROCKNIX ZIP."

case "$(uname -m)" in
    aarch64|arm64) ;;
    *) fail "This package needs aarch64 ROCKNIX (64-bit ARM userspace)." ;;
esac

export BIRDROMS_FIRMWARE=rocknix
export BIRDROMS_ROM_ROOT="${BIRDROMS_ROM_ROOT:-$(dirname -- "$BIRDROMS_PORTS_DIR")}"
[ -d "$BIRDROMS_ROM_ROOT" ] || fail "ROM storage is missing: $BIRDROMS_ROM_ROOT. No directories were created."
case "$BIRDROMS_ROM_ROOT" in
    /*) ;;
    *) fail "ROM storage must be an absolute path." ;;
esac
[ "$BIRDROMS_ROM_ROOT" != / ] || fail "ROM storage cannot be the system root."

# ROCKNIX installs PortMaster's controller helper in these locations. Its
# get_controls generates mappings from EmulationStation and selects compat libs.
BIRDROMS_CONTROL=""
for BIRDROMS_CANDIDATE in \
    "$BIRDROMS_PORTS_DIR/PortMaster/control.txt" \
    /storage/.config/PortMaster/control.txt \
    /roms/ports/PortMaster/control.txt; do
    if [ -f "$BIRDROMS_CANDIDATE" ]; then
        BIRDROMS_CONTROL="$BIRDROMS_CANDIDATE"
        break
    fi
done
if [ -n "$BIRDROMS_CONTROL" ]; then
    . "$BIRDROMS_CONTROL" || fail "Could not load the PortMaster controller helper."
    if declare -F get_controls >/dev/null; then
        get_controls || fail "PortMaster controller setup failed. Open PortMaster once and retry."
    fi
fi

# The helper can return success after mapper failure and leave an empty file.
# Do not let an unusable generated path suppress the firmware/SDL fallback.
valid_controller_file() {
    [ -r "$1" ] && LC_ALL=C grep -qE '^[[:space:]]*[[:xdigit:]]{32},[^,]+,' "$1"
}
if [ -n "${SDL_GAMECONTROLLERCONFIG_FILE:-}" ] && ! valid_controller_file "$SDL_GAMECONTROLLERCONFIG_FILE"; then
    unset SDL_GAMECONTROLLERCONFIG_FILE
    printf 'birdROMs: controller mapping file unavailable; trying firmware/SDL defaults.\n' >&2
fi

# PortMaster is optional: ROCKNIX also ships a controller database. Keep any
# mapping selected by get_controls or explicitly configured by the caller.
if [ -z "${SDL_GAMECONTROLLERCONFIG_FILE:-}" ]; then
    for BIRDROMS_DB in \
        /storage/.config/SDL-GameControllerDB/gamecontrollerdb.txt \
        /usr/config/SDL-GameControllerDB/gamecontrollerdb.txt; do
        if valid_controller_file "$BIRDROMS_DB"; then
            export SDL_GAMECONTROLLERCONFIG_FILE="$BIRDROMS_DB"
            break
        fi
    done
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
        fail "A required runtime library is missing; use this ZIP on ROCKNIX."
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
