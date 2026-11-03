# birdROMs for ROCKNIX — alpha

Extract `birdROMs.sh` and the complete `birdroms/` folder into ROCKNIX's **Ports**
folder. Keep them next to one another, then refresh the game list and launch
**birdROMs** from Ports. This is a 64-bit ARM (`aarch64`) package; hardware testing
is pending.

The app reads `birdroms/server.json` for its pairing server. If your downloaded
archive has no server URL, create or edit that file before launching:

```json
{"server_url":"https://your-birdroms-server.example"}
```

Approve the displayed pairing code using your server's web interface. The token
is saved in `birdroms/config.json` after pairing; keep that file when replacing
this alpha. Do not share it. The launcher never creates or replaces pairing
configuration.

ROMs go into system directories beneath the ROM root containing Ports. The app
preserves ROM filenames and skips files it recognizes as already present.
`BIRDROMS_ROM_ROOT` may override that root with another existing absolute
writable ROM directory. It must point to the ROM directory itself, not its card
mount. The app does not move existing ROMs or change emulator settings.

Controls: D-pad navigates, A opens/selects, B goes back, Y selects all/none,
L/R page, and Start downloads the selected ROMs. The firmware's Ports runner
handles returning to EmulationStation. In-app covers are supported; automatic
external frontend artwork integration is not part of this alpha.

Updates are manual for this alpha. Replace the distributed application files
while retaining `birdroms/config.json`. The app will not request the muOS update
package. Launch failures and runtime diagnostics are in `birdroms/birdroms.log`;
the launcher reports missing dependencies/storage before starting the app.

See the repository's `docs/platforms/rocknix.md` for build details, mapping
provenance, limitations and the hardware acceptance checklist.
