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

./"$APP_BIN" >"$APP_DIR/birdroms.log" 2>&1

[ -n "$CAFFEINE" ] && "$CAFFEINE" off
