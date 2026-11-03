# Device compatibility evidence

Only combinations with a recorded hardware result should be called supported.
An AArch64 build alone does not establish device or firmware compatibility.

| Client | Device/firmware | Build status | Hardware status |
| --- | --- | --- | --- |
| muOS | RG40XX-V, existing muOS client | Existing build/package checks pass | Existing device work is documented in `docs/b1-device-recon.md`; recheck after shared changes |
| ROCKNIX alpha | AArch64; Anbernic test device/release to be recorded | Host tests, cross-build and package checks pass | Pending SD-card setup and physical tests |
| Batocera | No target build yet | Planned | Unverified |
| KNULLI | No target build yet | Planned | Unverified |

ROCKNIX installation and the complete test checklist are in
[rocknix.md](rocknix.md). Record model/SoC, actual userspace architecture,
firmware version, filesystem/mount layout, app version/checksum, date, observed
results and limitations. Redact authentication tokens and device config files.
