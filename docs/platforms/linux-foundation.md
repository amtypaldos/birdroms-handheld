# Linux firmware profiles

Status: shared foundation implemented; ROCKNIX, Batocera and KNULLI runtime
support is not implemented by this change. Follow the
[rollout checklist](../superpowers/plans/2026-11-03-linux-platforms.md).

## Launcher and storage contracts

`BIRDROMS_FIRMWARE` defaults to `muos`. The other named targets are reserved
and rejected at startup until their package, mapping and updater are ready.
Unknown values fail instead of silently using the muOS updater.

The existing muOS launcher variables remain unchanged:

- `BIRDROMS_ROM_MOUNT`: existing card mount; ROMs go under `Roms/<muOS folder>`.
- `BIRDROMS_ROM_MOUNTS`: path-list of additional card mounts for existing-file
  detection; each uses the same `Roms/<muOS folder>` layout.

`firmware.Profile.ROMRoot` and `LibraryRoots` are complete ROM directories.
`download.NewWithProfile` never adds `Roms`. `SystemDirs` contains safe relative
system directories keyed by canonical server platform names. Unknown systems
use `_UNSORTED` and retain the existing UI warning (`Known=false`). Profiles are
copied into queues so later caller mutations cannot redirect queued downloads.
The legacy `download.New` and `SetLibraries` keep card-mount semantics.

Future launchers will supply `BIRDROMS_ROM_ROOT` once their resolver exists;
setting it today does not enable a firmware port. Keep target update identity
and architecture explicit when adding those resolvers.

## Storage safety

Storage roots must exist and be writable. The app checks this using a temporary
write probe, which is closed and removed, rather than relying on chmod support
on FAT/exFAT. It never creates an absent card mount. A missing muOS `Roms`
directory can be created beneath an existing card. `/` cannot be a muOS card.

Queues pin the initial storage directory identity and refuse to use a replaced
or removed mount. System-directory symlinks and non-regular partial files are
rejected. Writes use Go's rooted filesystem operations, which prevent escaping
the selected storage anchor. A new symlink replacing an initially absent muOS
`Roms` is rejected before a write probe or download. User configuration and
pairing are unchanged by this foundation.

Only muOS profiles enable external catalogue art. In-app covers are unchanged.
Self-update remains muOS-only until each new platform has its exact validated
package contract; no new target can fall back to the muOS artifact.

## Verification — 2026-11-03

- Existing internal-package baseline passed before implementation.
- `go test ./...`, `go vet ./...` and formatting checks passed with SDL2/ttf.
- Profile tests passed as UID 1000, including the real unwritable-root test.
- New regression cases cover exact lowercase roots, fallback, alternate-library
  skip, missing/replaced storage, unsafe mappings, system and partial-file
  symlinks, relative root-symlink insertion and system-root rejection.
- Independent review reproduced two storage issues; fixes passed both the
  repository tests and the reviewer's separate reproduction harness.
- Existing muOS build/package flow passed with Zig 0.17.0 and the existing
  verified sysroot. ELF needs SDL2, SDL2_ttf and libc; newest GLIBC symbol is
  2.36, below the existing 2.38 limit. Package layout and exclusion of runtime
  config/seed from the unpersonalized archive passed.
- No new firmware has been tested on hardware. This change publishes no new
  application release and does not deploy backend/frontend changes.
