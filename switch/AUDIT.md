# Source audit and port decisions

Reference revisions: handheld `9262c104a8af1e401fc50d80a2fd374e5d0462b9`;
backend `f9714553ddda4bcbacb192b137a0d7e8b478796d`.

Audited Go reference: internal/api/client.go, internal/pair/{pair,flow}.go,
internal/config/config.go, internal/browse/model.go, internal/download/{filename,queue}.go,
internal/platforms/{muos_folders.go,platform_aliases.csv}, ui/browse/browse.go,
ui/theme/theme.go and controller input. Backend reference cloned read-only into
../birdROMs-reference: backend/apps/accounts/{pairing,authentication}.py and
backend/apps/roms/{views,serializers,pagination}.py.

## API contract

- Public POST /api/devices/pair/start => 201 {code,qr_url,expires_at}.
- Public GET /api/devices/pair/poll?code=... => 202 pending, 200 device_token
  (delivered once), 410 expired/consumed, 429 backoff. Approval is on phone /pair.
- Authenticated reads use Authorization: Device TOKEN.
- GET /api/roms/platforms/ => string array.
- GET /api/playlists/ => unpaginated summaries; /api/playlists/ID/ => roms array.
- GET /api/roms/?fields=device&platform=...&page=N => count,next,previous,results.
- Device ROM: id,title,platform,extension,size_bytes,cover_url,filename,tags.
- GET /api/roms/ID/download/ streams original upload; cover_url is absolute.
- Cover authentication only to same origin; external images receive no device token.
- Backend download action returns plain FileResponse; Range handling is absent.
  The port restarts partial downloads rather than claiming resume support.

## Existing user behavior

Home groups playlists above platforms, then four-column covers. A toggles ROM
selection; B returns home; Y selects/deselects ALL ROMs in the current view,
loading remaining API pages first; L/R move a visible grid page; Start downloads
selected ROMs. Selection persists between views. No client search UI exists.
The Go grid lazily appends API pages instead of following server next URLs.
Dark background #0b0d17, cyan #00f0ff, pink #ff2d95, muted #8a90a8; Chakra Petch.

The Go client maps canonical names through platform_aliases.csv, never joins
server platform strings into filesystem paths, preserves original filenames,
sanitizes FAT-invalid characters, skips existing ROMs, uses .part plus rename,
and downloads sequentially. Its muOS folder names are unsuitable defaults for
RetroArch; Switch mappings are independently configurable and unknown platforms
use _UNSORTED. Archives remain archives.

## Platform differences and exclusions

Switch uses 1280x720, SD paths and native libnx controller input. Settings are
stored at sdmc:/switch/birdroms/config.json. Firmware, RetroArch configuration,
emulator cores and native Switch title installation are outside scope.
Go handheld self-update packages target muOS/ROCKNIX and cannot update a Switch
NRO; install new NRO versions manually. muOS gamelist refresh and box-art writes
are OS-specific and are not applicable to RetroArch v1 (no playlist changes).
