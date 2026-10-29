# Security policy

## Reporting a vulnerability

Please report security problems privately, not in a public issue. Use this repository's
**Security** tab, then **Report a vulnerability** (a private GitHub security advisory). Include what
you found, how to reproduce it, and the version (shown in the app's log, `birdroms.log`).

You can expect an acknowledgement within a few days. This is a volunteer project, so fixes happen
on a best-effort basis; please give us reasonable time to ship one before you disclose publicly.

## Scope

This repository is the on-device client. Things that matter here:

- the **device token** stored in `config.json` (it is written with mode `0600` and is read-only on
  the server side, but anyone who can read the file can browse and download as that device);
- the handling of server responses and file names while downloading (path traversal, writing
  outside the ROM folders);
- the `.muxapp` package layout and `mux_launch.sh`.

Problems in the birdROMs server belong in the server repository's security policy.

## Supported versions

Only the latest release receives fixes.
