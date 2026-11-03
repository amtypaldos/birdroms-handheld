# Linux Handheld Platforms Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Check off only verified work; do not begin implementation as part of the planning commit.

**Goal:** Release the existing birdROMs Linux client on ROCKNIX, Batocera and KNULLI through separate, reviewable platform branches.

**Architecture:** Keep the Go/SDL2 client and shared API/download/UI behavior. Add a narrow firmware profile boundary for roots, mappings, launch/update layout and external art. Use reproducible target builds; reuse a binary across firmware only after ABI verification.

**Tech Stack:** Existing Go, SDL2/SDL2_ttf, Linux AArch64, shell launchers, pinned cross-toolchains/sysroots, existing Django/React distribution service.

**Spec:** [Linux platform expansion](../specs/2026-11-03-linux-platforms.md).

## Global constraints

- Keep existing muOS packages, default API requests and Switch builds compatible.
- No token in packages, URLs or logs; `server.json` contains only `server_url`.
- Do not write runtime config until pairing succeeds; preserve writable config across updates.
- Validate TLS; stream ROMs; reject unsafe paths and archives; preserve original filenames.
- No RetroArch configuration/playlist changes, emulator installation or gamelist XML rewriting.
- Support claims require a recorded device/firmware test; ARM64 compilation alone is insufficient.
- Use `vYYYY.MM.DD[.N]` releases and one branch per platform; do not combine pending platforms.

## Review focus

1. Wrong firmware/update artifact: explicit target selection must reject mismatches before writing.
2. Missing/remapped storage: never create a ROM tree in the firmware partition or wrong mount.
3. Existing token/config on FAT/exFAT: update and subsequent save must preserve pairing and settings.
4. Unknown systems and hostile filenames: keep output within the selected ROM root with a visible fallback.
5. Frontend/controller lifecycle: repeated start/exit and failed downloads must restore normal navigation.

## Current source audit (2026-11-03)

Main currently documents muOS RG40XX-V first; the Switch work remains on
`feature/nintendo-switch` / PR #14. This plan does not merge that feature.

- `justfile` and `scripts/ci-sysroot.sh`: muOS dynamic ARM64 build targeting
  `aarch64-linux-gnu.2.38`. This is not a universal Linux compatibility floor.
- `cmd/birdroms/main.go`: 640x480 UI, relative config/font/seed paths, launcher
  environment for ROM storage, muOS catalogue-art integration and self-update.
- `internal/download/queue.go`: constructs `<mount>/Roms/<system>` and calls
  `platforms.Folder`; alternate-library checks must follow the same profile.
- `internal/platforms/muos_folders.go`: canonical name to muOS folder lookup.
  `platform_aliases.csv` includes cross-firmware aliases but its `muos_folder`
  column is not a ROCKNIX/Batocera directory mapping.
- `package/mux_launch.sh`: muOS mount discovery, launch environment and restart.
- `internal/update/update.go`: extracts the `birdROMs/` `.muxapp` layout.
- `internal/api/client.go`: existing handheld metadata/download requests default
  to muOS. Add target-aware methods without breaking these methods.
- Backend distribution changes belong in the birdROMs backend repository, not
  a copied backend inside this repository. Coordinate matching feature branches.

Implementation evidence: [Linux foundation](../../platforms/linux-foundation.md).

## Tracker

- [x] Inspect source and document design/rollout on main.
- [ ] Foundation merged; existing muOS behavior verified.
- [ ] ROCKNIX package/build complete.
- [ ] ROCKNIX server/page integration complete.
- [ ] ROCKNIX hardware acceptance recorded and release published.
- [ ] Batocera package/build complete.
- [ ] Batocera server/page integration complete.
- [ ] Batocera hardware acceptance recorded and release published.
- [ ] KNULLI package/build complete.
- [ ] KNULLI server/page integration complete.
- [ ] KNULLI hardware acceptance recorded and release published.

## Task 1 — Shared Linux profile boundary

**Branch:** `feature/linux-platform-profiles`, from current main.

**Files:** create `internal/firmware/profile.go`, `profile_test.go`; modify
`internal/download/queue.go`, `match.go` as required by actual path calls,
`internal/platforms/muos_folders.go`, `cmd/birdroms/main.go`, and their tests.
Keep the existing muOS mapping table and `Folder` API compatible.

**Interfaces:** introduce `firmware.Target` string (`muos`, `rocknix`, `batocera`,
`knulli`); `firmware.Profile` with `Target`, `ROMRoot`, `LibraryRoots`,
`SystemDirs` (canonical platform -> safe relative directory) and
`ExternalArt` (`muos` or `none`). Define
`firmware.Resolve(target Target, getenv func(string) string) (Profile, error)`
and `(Profile).Folder(platform string) (string, bool)`.
Add `download.NewWithProfile(f Fetcher, profile firmware.Profile, roms []api.ROM) *Queue`;
keep `New` as a muOS-compatible wrapper. ROMRoot is already the ROM directory,
so the new constructor never unconditionally appends `Roms`.

- [x] Add table tests showing muOS NES still lands under `<mount>/Roms/FC`,
  a synthetic lowercase profile lands under `<root>/nes`, alternate-library
  detection uses the same mapping, and unknown names use `_UNSORTED`.
- [x] Test profile rejection of unknown target, empty root, traversal/absolute
  mappings and missing/unmounted storage. An absent mount must not be silently
  created; test unwritable storage as a non-root user.
- [x] Run the targeted tests before implementation and confirm new cases fail.
- [x] Implement the profile boundary and queue compatibility wrapper. Select
  the firmware via `BIRDROMS_FIRMWARE`; unset preserves muOS. Non-muOS launchers
  supply `BIRDROMS_ROM_ROOT`; retain existing muOS mount variables unchanged.
- [x] Gate external art by profile; test that non-muOS downloads never access
  muOS catalogue paths. Keep in-app cover display unchanged.
- [x] Run `go test ./internal/...`, `go test ./...` with SDL dependencies, and
  the existing muOS build/package/glibc checks. Inspect the package layout.
- [ ] Merge the reviewed foundation PR after CI passes. Update this tracker.

## Task 2 — ROCKNIX build, mapping and launcher

**Branch:** `feature/rocknix`, after Task 1 merges.

**Files:** create `package/rocknix/birdROMs.sh`, `package/rocknix/README.md`,
`scripts/build-linux.sh`, `scripts/package-linux.sh`,
`scripts/test-linux-package.py`, `internal/firmware/rocknix.go` and tests,
`docs/platforms/rocknix.md`; update `justfile` and CI/release workflows.

**Interfaces:** build script accepts firmware target and version and emits
`dist/<target>/birdroms`; package script emits `birdROMs-rocknix-aarch64.zip`
plus SHA-256. Launcher sets the profile/root, changes to its quoted app directory
and forwards a usable exit status. Archive includes launcher, binary, fonts,
licenses and a manifest identifying target, architecture, version, layout and
binary hash. Exact manifest schema is frozen with Task 3 before publication.

- [ ] Record the user's model and installed ROCKNIX version; inspect actual
  `uname`, userspace ELF architecture, libc, SDL2/ttf, controller GUIDs, display
  driver, mount layout and launch environment. Save redacted facts in docs.
- [ ] Read that version's firmware/system configuration and PortMaster runtime
  contracts. Confirm supported launcher location and system directories; add
  mapping provenance. Do not derive folders from the muOS CSV column.
- [ ] Add failing mapping tests for NES/SNES/GB/GBA/Genesis/PSX, absent systems,
  unknown names and safe overrides. Add package tests for exact layout,
  executable launcher, spaces in paths, missing dependencies and no secrets.
- [ ] Establish pinned reproducible sysroot/toolchain based on collected ABI
  requirements; inspect ELF interpreter, dependencies and maximum GLIBC/GLIBCXX
  symbols. Do not reuse the muOS glibc 2.38 build without evidence.
- [ ] Implement mappings, launcher and package; use supported PortMaster
  environment helpers only where confirmed. Do not globally stop frontend
  services with a copied command from another device.
- [ ] Test launcher fixtures with fake app/helper commands: normal exit,
  cancellation, app crash and repeated launch all restore required state.
- [ ] Run shared tests, package tests and clean target build. Publish CI
  artifacts as hardware-pending; retain existing muOS/Switch release jobs.
- [ ] Complete Tasks 3 and 4 for ROCKNIX before marking supported or releasing.

## Task 3 — Target-specific self-hosted distribution and updates

**Branches:** current platform branch here, matching `feature/<target>-distribution`
in backend. Repeat for each new target; share validated publisher infrastructure.

**Files here:** `internal/api/client.go`, `internal/api/handheld_test.go`,
`internal/update/update.go`, update tests, target launcher and package scripts.
**Backend:** inspect actual handheld views/serializers/publisher/installer,
container build workflow and `frontend/src` Handheld component before editing;
record exact paths in the platform PR. Preserve default muOS and Switch contracts.

**Interfaces:** introduce explicit target-aware metadata/download methods while
keeping existing muOS methods. Use `platform=rocknix`, `batocera` or `knulli`
and `arch=aarch64` for the new contracts; unsupported combinations return 400,
unpublished valid combinations return unavailable. Define schema jointly before
coding: firmware, architecture, version, archive identity and binary size/hash.
The personalized archive hash differs from the stored archive; verify immutable
manifest/binary content rather than comparing personalized bytes to stored ZIP.

- [ ] Add failing API tests proving exact target/architecture query selection,
  legacy muOS behavior, unsupported combination rejection and unavailable state.
- [ ] Add publisher/package/stager contract fixtures for all three identities;
  wrong firmware/architecture, malicious/duplicate entries, symlinks, oversized
  archives, invalid hashes and version mismatch must fail before installation.
- [ ] Implement per-target publication, token-free seed personalization and
  container bundling with pinned versions. Backend changes get their own tests.
- [ ] Extend staging and launcher install/rollback without touching runtime
  config. Test interruption before/after swap, failed rename, insufficient
  storage, and preservation of existing config followed by a successful save.
- [ ] Keep assets/launcher versions coherent with the binary. Preserve failed
  update recovery and prevent automatic retry loops; never install another
  target as a fallback.
- [ ] Add target cards and accurate install/control/update instructions to the
  existing Handheld page; label unverified targets experimental. Test stale
  platform requests and error states; check desktop/mobile with Playwright.
- [ ] Run backend/frontend/client suites and test the actual built container:
  authenticated metadata -> personalized ZIP -> native stager -> restart fixture.
- [ ] Commit/review both repositories. Publish/deploy only after compatible
  server/client contracts pass; do not expose a download for a missing artifact.

## Task 4 — Platform hardware acceptance and release

**Files:** create `docs/platforms/compatibility.md`, per-platform device test
records under `docs/platforms/testing/`; update `README.md`, `docs/install.md`,
`docs/publishing.md` and this tracker. Run this gate for each target separately.

**Evidence interface:** each record contains device model/SoC, actual userspace
architecture, firmware version, filesystem/mounts, app version/checksum,
test date, observed results and outstanding limitations. Never include tokens.

- [ ] Install from the self-hosted ZIP on a fresh app directory; confirm the
  seed URL, pairing flow and absence of premature runtime config writes.
- [ ] Test platforms/playlists, covers, search if present, paging, multi-page
  selection, select-all/none and download progress with physical controls.
- [ ] Download representative systems to their real firmware directories;
  preserve filenames/extensions, skip existing files across configured roots,
  test unknown-platform warning and verify games appear after the documented
  native game-list refresh. Do not require writing gamelist XML.
- [ ] Test no network, revoked token, cancelled/failed download, `.part` behavior,
  storage-full/write errors and normal return to the frontend. Verify display
  legibility/rotation, sticks/D-pad, suspend behavior and repeated launch/exit.
- [ ] Install a newer self-hosted build; verify exact target identity, restart,
  rollback recovery and unchanged token/ROM settings. Save config afterward.
- [ ] Revalidate muOS package/default API and existing Switch release outputs;
  distinguish automated build results from physical runtime evidence.
- [ ] Review platform PRs, merge, publish versioned artifacts/checksums and pin
  matching server images. Mark only tested combinations supported; document
  other combinations as unverified. Link evidence and tick platform tracker.

## Task 5 — Batocera AArch64

**Branch:** `feature/batocera`, from main after ROCKNIX acceptance/integration.

**Files:** create `internal/firmware/batocera.go`, matching tests,
`package/batocera/birdROMs.sh`, launcher tests,
`docs/platforms/batocera.md`; extend shared build/package scripts and CI.

**Interfaces:** reuse Task 1 profile and Task 3 distribution contract with
`platform=batocera&arch=aarch64`; artifact `birdROMs-batocera-aarch64.zip`.
Default ROM root is `/userdata/roms` after verifying that writable mount on the
tested installation. Launcher lives in the supported Ports location; runtime
config belongs to a persistent writable app directory, not a read-only root.

- [ ] Verify a Batocera release/image actually supports the user's exact model
  before flashing/testing; keep a backup of existing cards and configuration.
- [ ] Repeat Task 2 runtime/ABI reconnaissance and add Batocera-specific mapping
  tests from its system definitions. Prove paths are lowercase where required;
  root must not become `/userdata/roms/Roms`.
- [ ] Add failing launcher/package tests for read-only system partition,
  userdata on alternate storage, spaces, crash/exit and frontend restoration.
- [ ] Implement target profile/build/Ports launcher; share ROCKNIX binary only
  if runtime ABI checks pass, otherwise build with a separate compatible sysroot.
- [ ] Document game-list refresh using tested Batocera UI behavior. External
  frontend art/gamelist rewriting remains outside this port's initial scope.
- [ ] Complete Task 3 distribution/page integration and Task 4 hardware gate.
- [ ] Merge/release independently and update tracker before starting KNULLI.

## Task 6 — KNULLI AArch64

**Branch:** `feature/knulli`, from updated main.

**Files:** create `internal/firmware/knulli.go`, matching tests,
`package/knulli/birdROMs.sh`, launcher tests,
`docs/platforms/knulli.md`; extend shared build/package scripts and CI.

**Interfaces:** reuse profile/distribution contracts with
`platform=knulli&arch=aarch64`; artifact `birdROMs-knulli-aarch64.zip`.
Keep separate target identity even if a tested binary or mapping table is shared.

- [ ] Confirm device support and firmware release; inspect actual userdata,
  PortMaster installation/helper behavior, filesystem and SDL/runtime ABI.
- [ ] Add failing tests for target identity, expected folder mappings, missing
  helpers, alternate storage, launcher exit and config update/save preservation.
- [ ] Implement only KNULLI differences, referencing its current supported
  launcher mechanism. Do not assume Batocera/ROCKNIX helper availability.
- [ ] Complete Task 3 and Task 4 independently. Record differences or limitations
  such as filesystem-specific runtime requirements based on actual evidence.
- [ ] Merge/release and update tracker and compatibility matrix.

## Future platform intake

ArkOS and AmberELEC can follow separate branches using the same gates after the
three targets above. Select them only when there is a tester and a supported
image for an available device. Batocera x86_64 and Steam Deck require explicit
architecture artifacts/CI; never label an AArch64 package universal. Desktop and
Android packaging are not prerequisites for these handheld releases.

## Completion and handoff

This planning change modifies documentation only. Implementation begins in the
foundation branch in a later task. For each platform, replace unchecked items
with checked items only when linked evidence exists; leave hardware-pending
items open. Add discoveries and exact firmware facts to the platform document
instead of silently weakening the requirements.

Final platform deliverables: source/profile, launcher, reproducible build,
versioned archive/checksum, server/container support, frontend install docs,
automated test record, hardware matrix and recovery instructions.
