#!/bin/sh
# HELP: birdROMs - download ROMs shared by your friends
# ICON: birdroms
# GRID: birdROMs

. /opt/muos/script/var/stage.sh
. /opt/muos/script/var/retro.sh

SETUP_STAGE_OVERLAY

APP_BIN="birdroms"
SETUP_APP "$APP_BIN" ""

# -----------------------------------------------------------------------------

APP_DIR="${1:-$(dirname "$0")}"
cd "$APP_DIR" || exit

export SDL_GAMECONTROLLERCONFIG_FILE="/usr/lib/gamecontrollerdb.txt"

# Where ROMs go. muOS's "ROM storage" setting can point at one card (internal)
# while the library lives on another (the SD card), and its game list merges
# them all. Download to the first storage that already has a ROM library (SD
# card first), else to muOS's configured one, and let the app look for
# already-downloaded ROMs on every storage.
ROM_MOUNT="$(GET_VAR "device" "storage/rom/mount" 2>/dev/null || true)"
SD_MOUNT="$(GET_VAR "device" "storage/sdcard/mount" 2>/dev/null || true)"
MOUNTS=""
PRIMARY=""
for M in "$SD_MOUNT" "$ROM_MOUNT"; do
    [ -n "$M" ] || continue
    case ":$MOUNTS:" in *":$M:"*) continue ;; esac
    MOUNTS="${MOUNTS:+$MOUNTS:}$M"
    if [ -z "$PRIMARY" ] && { [ -d "$M/Roms" ] || [ -d "$M/ROMS" ]; }; then PRIMARY="$M"; fi
done
[ -n "$PRIMARY" ] || PRIMARY="$ROM_MOUNT"
[ -n "$PRIMARY" ] && export BIRDROMS_ROM_MOUNT="$PRIMARY"
[ -n "$MOUNTS" ] && export BIRDROMS_ROM_MOUNTS="$MOUNTS"

CAFFEINE="$(command -v CAFFEINE 2>/dev/null || true)"
[ -n "$CAFFEINE" ] && "$CAFFEINE" on

# >>> self-update
# The app updates itself from the server it is paired with: it unpacks the new
# package beside this script, writes the executable to birdroms.new and the
# server's package hash to update.sha256, then exits 0. A running binary cannot
# replace itself, so the swap happens here, before launch (a previous run staged
# an update and died) and after the app exits. Both files must exist; a failed
# or cancelled update leaves neither, so the installed app starts unchanged.
apply_update() {
    [ -f "birdroms.new" ] && [ -f "update.sha256" ] || return 1
    chmod +x "birdroms.new"
    mv -f "birdroms.new" "$APP_BIN" || return 1
    mv -f "update.sha256" "installed.sha256"
}

# Leftovers of an update that did not complete (half-unpacked, cancelled, killed).
drop_stray_update() {
    [ -f "birdroms.new" ] && [ -f "update.sha256" ] && return 0
    rm -f "birdroms.new" "update.sha256" .update-*.part
}

launch_app() {
    LOG="$APP_DIR/birdroms.log"
    apply_update
    drop_stray_update
    ./"$APP_BIN" >"$LOG" 2>&1
    # The app exited: restart once into a version it just staged (no loop).
    if apply_update; then
        ./"$APP_BIN" >>"$LOG" 2>&1
    fi
    drop_stray_update
}
# <<< self-update

launch_app

[ -n "$CAFFEINE" ] && "$CAFFEINE" off
