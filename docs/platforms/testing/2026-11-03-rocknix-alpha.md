# ROCKNIX alpha release verification — 2026-11-03

Release: [rocknix-v2026.11.03-alpha.1](https://github.com/amtypaldos/birdroms-handheld/releases/tag/rocknix-v2026.11.03-alpha.1).
Source was merged in [PR #16](https://github.com/amtypaldos/birdroms-handheld/pull/16)
as `2d09917e8553fffb5d305023b8cb220b6723ad0d` before tagging. Tag ancestry on
main was checked. The release is a prerelease; the regular latest release
remained `v2026.11.02` after publication.

## Downloaded published artifact

- `birdROMs-rocknix-aarch64.zip`: **3,019,805 bytes**.
- ZIP SHA-256: `cc6cab0950c94c56dd0878f00c2a076184578afd8e221bd78562ecd4077ea152`.
- Contained native binary: **7,576,992 bytes**.
- Binary SHA-256: `5e330812b0777c699eb6025c4d7c5ff2f1cd262f4e72a506bb981556da6bdb85`.

Downloaded from GitHub Releases and verified against the published checksum,
embedded manifest and actual ELF bytes. Target/version/architecture are
`rocknix` / `2026.11.03-alpha.1` / `aarch64`. URL-only seed is
`https://retro.birdperson.net`. No runtime config or device token is packaged.
Published ELF requires GLIBC_2.17; its dynamic dependencies are SDL2, SDL2_ttf,
libc, libresolv and libpthread. Runtime libraries are supplied by ROCKNIX.

## Automated evidence

- Full Go/SDL tests, formatting and vet passed locally and in GitHub CI.
- Twelve launcher tests and four package CLI tests passed.
- Viewport and shoulder controller regression tests passed.
- Pinned sysroot/cross-build, AArch64/interpreter and glibc checks passed.
- Actionlint and shell syntax checks passed.
- Independent review completed; controller-helper fallback issue fixed and
  rechecked with missing, empty and comment-only mapping fixtures.
- Existing muOS package builds passed alongside the new target.
- Both push/PR ROCKNIX workflows passed; tagged publication workflow
  [41735293625](https://github.com/amtypaldos/birdroms-handheld/actions/runs/41735293625)
  completed successfully.

## Still pending

No physical ROCKNIX device was used. Exact test model/firmware/filesystem,
launch/display/controller behavior, actual SD downloads and frontend return must
be recorded using the [hardware checklist](../rocknix.md#physical-test-checklist).
Self-hosted ROCKNIX archive distribution and automatic updates remain a separate
milestone. This alpha uses manual GitHub ZIP installation and cannot apply a
muOS update package.
