# Validation results — 2026-10-31

Source implementation: commit `0bb1668` on `feature/nintendo-switch`.

## Regular Switch release v0.1.0

The user reported the application "fully works now" on physical Nintendo Switch
hardware after alpha.3. This is user-reported hardware validation, not a claim
that every device/mode and failure scenario in the manual checklist was tested.

The application code is unchanged from `8d5aa59` (alpha.3); only the packaged
version metadata changes to 0.1.0. All six host suites passed for that source.
A fresh clean Switch build passed with no compiler warnings; NRO/icon/NACP/RomFS
layout validation passed. The manual checklist remains for broader testing.

Stable artifact: **10,357,447 bytes**.

SHA-256: `da1faf6a3186bd8a1f00673380a3099565e2095b0cb760dc4f1acdb339bbd8d8`.

The following alpha sections are historical validation records for earlier
artifacts. Their pending-hardware notes and checksums refer to those releases.

## Alpha.3: persist only after receiving the token

Per user requirement, server URL entry and the entire pending pairing flow now
remain in memory. No configuration save occurs until approval returns a device
token. Revoked tokens are cleared in memory; the existing config stays unchanged
until a new token arrives. Failed saves retain the received token for A to retry.

The UI uses a guarded `save_paired_config` entry point at its two remaining save
locations (approval and save retry). Regression tests verify that saving a
server-only draft cannot create even the containing directory, and cannot
change an existing config or create staging/backup files. Alpha.2's recovery and
Switch rename fixes remain in place for users with an existing config.

All six host suites passed; clean Switch build and NRO layout checks passed.
Hardware retest is pending.

Alpha.3 artifact: **10,357,447 bytes**.

SHA-256: `bade9d3dc9c667d549c96ca5e475c2a1186afeae739cfb717e08a207a1570f0f`.

## Alpha.2 pairing persistence fix

Hardware feedback from the first prerelease: initial server config writes
succeeded, pairing delivered a token, but replacing config.json failed.
libnx v4.12.0 `nx/source/runtime/devices/fs_dev.c` forwards rename directly to
`fsFsRenameFile`, which reports EEXIST for an occupied destination. Linux rename
replaces the destination, so the original host tests missed this difference.

The fix keeps the token and serialized config in memory, flushes a staging file,
uses an old-config backup when replacement is unsupported, restores on failure,
and recovers valid staged/backup files on the next launch. A retries failed saves
without requesting a new token.

Clean Switch build and all **six** host suites passed after the fix. New tests
wrap Linux rename to reproduce Switch semantics and cover:

- Server-only config followed by saving the approved device token.
- Failed final rename restoring the previous config.
- Interrupted replacement recovering the staged config.
- Corrupt staged config recovering the backup.
- Failed install plus rollback preserving recovery files.
- Standard POSIX replacement remaining supported.

Alpha.2 artifact: **10,357,447 bytes**.

SHA-256: `4d1f70b713d3ba8c1cd5694f23265268ffd1b059bbd668755388908066291709`.

NRO structure validation passed. This persistence fix has not yet been retested
on physical hardware. The following sections retain the original alpha.1 build
and test record; their older size/checksum refer to that release.

## Switch build

Cleaned output, then ran `./switch/scripts/build.sh -j2` successfully with the
pinned official devkitPro image. No compiler warnings or errors in the final log.

- devkitA64 r29.2-1
- libnx 4.12.0-1
- SDL2 2.28.5-4, SDL2_image 2.0.4-1, SDL2_ttf 2.22.0-2
- switch-curl 7.69.1-5 (native libnx SSL, system certificate store)
- Artifact: `switch/birdroms.nro`, **10,353,351 bytes**
- SHA-256: `a7b947be7c35ce694d84ca9f7c138dfc99d64d8d573214222dca1a7425ccfcdd`
- `python3 switch/scripts/validate-nro.py`: valid NRO layout with embedded
  Homebrew Menu icon, NACP metadata and RomFS assets.

The required minimal SDL2/libnx NRO milestone succeeded before implementation
and delegation (commit `de528d7`). No hardware execution is implied by compilation.

## Linux host tests

`./switch/scripts/test.sh` succeeded against the final implementation:

1. API JSON, optional fields, pagination, pairing statuses, malformed responses,
   expiry validation and unsafe token rejection.
2. Real local HTTP fixture: authentication headers, platform/playlist/listing
   calls, URL encoding, pairing, cross-origin cover header suppression, redirect
   rejection, image byte cap, streaming, HTTP failures and cancellation.
3. Config/storage/download tests: roundtrip/malformed config, platform aliases,
   filename safety, traversal/symlink rejection, existing files, collision
   preservation, `.part` retention/restart, short bodies, cancellation, and
   revoked authentication stopping the queue with the failed item recorded.
4. UI selection/global deduplication, selecting across appended pages, focus and
   grid scrolling.
5. Predecode PNG/JPEG dimensions: valid headers, truncation and oversized images.

Output:

```
API JSON/pagination/pairing tests passed
API HTTP/auth/streaming/cancellation tests passed
Storage tests passed: configuration, mappings, filename/path safety, symlinks, existing detection, queue failures, partial restart, collisions
All Switch host test suites passed
```

## Existing Go regression tests

`docker run --rm -v "$PWD:/work:ro" -w /work golang:1.26 go test ./internal/...`
passed all 11 internal packages: api, boxart, browse, config, download, input,
muxapp, pair, platforms, update and version. No existing Go source, release
pipeline or backend source was modified. The Go SDL release build was not rerun.

## Not verified

No physical Switch, live birdROMs server, real SD card, Joy-Con/Pro Controller,
software keyboard, native TLS connection or suspend/resume operation was tested.
Host network tests use a deterministic HTTP fixture, not an authenticated live
server. Native SSL certificate validation remains enabled, with no bypass.

See README.md for manual installation/configuration and the physical testing
checklist. Known differences include manual updates, unsupported WebP covers,
configuration-file folder editing, and restarting partial downloads because the
backend does not implement HTTP Range handling.
