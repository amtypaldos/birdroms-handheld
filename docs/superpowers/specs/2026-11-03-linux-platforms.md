# Linux handheld platform expansion

## Goal and scope

Extend the existing Go/SDL2 client to ROCKNIX, Batocera and KNULLI, preserving
muOS and the separate native Switch implementation. Ship one shared Linux
client codebase with explicit firmware profiles and independently validated
packages. Share executable builds only when their ABI and runtime requirements
are proven compatible; AArch64 alone is insufficient.

The user can test Anbernic devices on muOS and ROCKNIX and install other
compatible firmware. Exact models and installed firmware versions must be
recorded before hardware validation. Do not imply that every Anbernic model
can run every firmware, or that a successful build establishes hardware support.

## Required behavior

- Preserve pairing, token persistence, playlists/platforms, pagination, cover
  grid, selection, original filenames, sequential downloads, cancellation,
  existing-file checks and self-hosted update behavior.
- Preserve existing muOS paths, aliases, catalogue art, launcher and `.muxapp`.
- Firmware-specific ROM roots and system-directory mappings must be explicit,
  validated and independent of backend platform display names.
- ROM roots are writable user-data locations, never firmware/system partitions.
  Unknown platforms go to a visible `_UNSORTED` directory with a warning.
- Keep authentication/configuration outside update payloads. A personalized
  package may contain `server.json` with only `server_url`; persist runtime
  configuration only after receiving the pairing token. Preserve existing
  config and ensure it remains writable after updates.
- Updates must select the exact firmware/architecture package, validate it,
  stage safely and install after process exit with rollback. An unpublished
  target means no update; never fall back to a muOS or Switch artifact.
- Validate HTTPS certificates, avoid logging tokens, bound archive extraction,
  reject traversal/symlinks and preserve completed ROMs on failures.
- Keep the UI controller operated and responsive across supported resolutions.
- Do not edit RetroArch settings, generate emulator playlists, rewrite
  EmulationStation gamelist XML or install emulator cores. Document the native
  frontend's game-list refresh action; use automation only if safely verified.
- In-app covers remain supported. External frontend art is muOS-only initially;
  do not apply its catalogue paths to other firmware or promise scraping.
- Deliver versioned archives/checksums, reproducible builds, self-hosted server
  publication/container integration, accurate frontend docs and hardware checks
  for each supported target. PortMaster catalogue acceptance is a separate
  optional submission, not a dependency of manual installation.

## Rollout and status

| Work | Branch | Initial status |
| --- | --- | --- |
| Linux profile/build foundation | `feature/linux-platform-profiles` | Merged: PR #15; automated muOS regression/build checks passed |
| muOS regression gate | foundation + every platform branch | Existing implementation; revalidate |
| ROCKNIX AArch64 | `feature/rocknix` | Alpha merged/published (PR #16); hardware and self-hosted distribution pending |
| Batocera AArch64 | `feature/batocera` | Planned; distinct launcher/package |
| KNULLI AArch64 | `feature/knulli` | Planned; distinct firmware validation |
| ArkOS / AmberELEC | future separate branches | Candidates; not committed support |
| Batocera x86_64 / Linux desktop / Steam Deck | future separate branches | Deferred architectures/packaging |
| Android / Windows / macOS | future separate projects within repo | Outside this rollout |

Merge each accepted branch into main before starting the next dependent branch.
A platform can be "build verified / hardware pending" without being marked
supported. Keep a matrix of model, SoC, userspace architecture, firmware version,
filesystem, build tag and evidence. Release tags use `vYYYY.MM.DD[.N]`.

## Authoritative references

Read the target firmware's current source/configuration during implementation;
these links explain intended integration, not guarantees for an untested model.

- [ROCKNIX Ports](https://rocknix.org/systems/ports/)
- [PortMaster installation](https://portmaster.games/installation.html)
- [PortMaster build environments](https://portmaster.games/build-environments.html)
- [Batocera Ports](https://wiki.batocera.org/systems:ports)
- [Batocera file management](https://wiki.batocera.org/file_management)
- [KNULLI quick start](https://knulli.org/play/quick-start/)
- [KNULLI major updates](https://knulli.org/guides/major-updates/)

ROCKNIX commonly exposes ports under `/roms/ports`; Batocera uses
`/userdata/roms/ports`. Resolve actual storage on each firmware/device rather
than guessing from these examples. KNULLI is a separate compatibility target;
do not assume upstream Batocera's runtime or PortMaster integration is identical.
