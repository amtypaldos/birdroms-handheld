#!/usr/bin/env bash
# Fail if a binary needs a glibc newer than muOS can provide (default 2.38).
#
#   scripts/check-glibc.sh dist/birdroms [max-version]
#
# Prints the shared libraries the binary asks for and the newest GLIBC_x.y symbol version it
# references. Uses readelf when available (Linux CI), else falls back to `strings` (macOS).
set -euo pipefail
bin="${1:?usage: check-glibc.sh <binary> [max-glibc]}"
max="${2:-2.38}"

echo "== shared libraries (DT_NEEDED)"
if command -v readelf >/dev/null; then
  readelf -d "$bin" | awk '/NEEDED/ { gsub(/[\[\]]/, "", $5); print "  " $5 }'
  versions="$(readelf --version-info "$bin" | grep -o 'GLIBC_[0-9][0-9.]*' | sort -u)"
else
  strings -a "$bin" | grep -E '^lib[A-Za-z0-9_+.-]+\.so(\.[0-9]+)*$' | sort -u | sed 's/^/  /'
  versions="$(strings -a "$bin" | grep -o '^GLIBC_[0-9][0-9.]*$' | sort -u)"
fi

echo "== GLIBC symbol versions"
echo "$versions" | sed 's/^/  /'
newest="$(echo "$versions" | sed 's/GLIBC_//' | sort -V | tail -1)"
echo "== newest: GLIBC_${newest:-none} (limit $max)"
if [ -n "$newest" ] && [ "$(printf '%s\n%s\n' "$newest" "$max" | sort -V | tail -1)" != "$max" ]; then
  echo "check-glibc: $bin needs glibc $newest, newer than $max" >&2
  exit 1
fi
