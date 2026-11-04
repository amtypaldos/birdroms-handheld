# Batocera alpha verification — 2026-11-04

Release: [batocera-v2026.11.04-alpha.1](https://github.com/amtypaldos/birdroms-handheld/releases/tag/batocera-v2026.11.04-alpha.1).
Source merged in [PR #17](https://github.com/amtypaldos/birdroms-handheld/pull/17)
as `70e39251f8680d3ea60a23b6f4c8bdd4f5b122c0` before tagging. Main ancestry
was verified. The release is a prerelease; regular latest remains `v2026.11.02`.

## Published download

- `birdROMs-batocera-aarch64.zip`: **3,020,537 bytes**.
- ZIP SHA-256: `b91f9e3a0391e889d7477cee81f11f26a4c3a65663d0e3257bff0aa9509b0c4c`.
- Binary size: **7,581,984 bytes**.
- Binary SHA-256: `36576bf88e56a5785fb7b6b2c7b1b357728f2f3407bb685a27689d5487573a82`.

The actual release asset was downloaded and checked against its checksum and
manifest. Version/target/architecture, AArch64 ELF bytes, URL-only server seed
and absence of runtime config were verified. The seed is
`https://retro.birdperson.net`; no device token is included. Published ELF
requires only GLIBC_2.17 symbol versions and links device-supplied SDL2/SDL2_ttf,
libc, libresolv and libpthread.

## Evidence and limits

Full Go/SDL tests and vet passed.13Batocera launcher fixtures,12ROCKNIX launcher
fixtures and5packager CLI tests passed. New tests cover91 source-verified
mappings, strict overrides, alternate storage, original-filename download paths
and Batocera viewport scaling. Non-root permission tests passed. Source review
found no important issues. Shell syntax and Actionlint passed.

All PR checks passed, including both existing muOS package builds and the
ROCKNIX regression package build. Tagged Batocera publication workflow
[41765885738](https://github.com/amtypaldos/birdroms-handheld/actions/runs/41765885738)
passed.

Hardware testing was explicitly deferred by the user. No physical model/image
or SD-card runtime is verified; see the [acceptance checklist](../batocera.md#physical-acceptance-checklist).
Self-hosted Batocera archive distribution and automatic updates remain pending.
