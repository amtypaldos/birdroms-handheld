# birdROMs for Batocera — alpha

Extract `birdROMs.sh` and the complete `birdroms/` folder into
`/userdata/roms/ports/`. Keep the script beside the folder. Update the game list
and launch birdROMs from **Ports**. This is a 64-bit ARM (`aarch64`) package;
physical hardware testing is pending.

Pairing uses `birdroms/server.json`:

```json
{"server_url":"https://your-birdroms-server.example"}
```

If the archive already contains your server's URL, launch and approve the
pairing code in its web interface. The runtime token is written to
`birdroms/config.json` after pairing. Keep that file when manually updating the
alpha; do not share it.

The launcher preserves the SDL controller/display environment supplied by
Batocera's normal Ports runner. Configure your controller in EmulationStation
before launching. It also accepts Batocera's generated
`/tmp/gamecontrollerdb.txt`. A missing controller configuration produces a
warning rather than guessed button bindings. No PortMaster installation is
required or sourced.

ROMs download to mapped system directories beneath the ROM root containing
Ports, normally `/userdata/roms`. Set `BIRDROMS_ROM_ROOT` only to override this
with another existing absolute ROM directory. Original filenames are retained;
unknown platforms use `_UNSORTED`. Refresh the game list using Batocera's normal
menu after downloading. The app does not modify RetroArch or gamelist XML.

D-pad navigates, A opens/selects, B goes back, Y selects all/none, L/R page and
Start downloads selections. In-app covers are supported. External game-list
artwork and self-hosted app updates are not part of this alpha; update manually
while retaining runtime config. Startup/runtime diagnostics are recorded in
`birdroms/birdroms.log`.
