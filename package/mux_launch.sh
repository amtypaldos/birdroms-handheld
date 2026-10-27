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

ROM_MOUNT="$(GET_VAR "device" "storage/rom/mount" 2>/dev/null || true)"
[ -n "$ROM_MOUNT" ] && export BIRDROMS_ROM_MOUNT="$ROM_MOUNT"

CAFFEINE="$(command -v CAFFEINE 2>/dev/null || true)"
[ -n "$CAFFEINE" ] && "$CAFFEINE" on

./"$APP_BIN" >"$APP_DIR/birdroms.log" 2>&1

[ -n "$CAFFEINE" ] && "$CAFFEINE" off
