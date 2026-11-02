#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p tests/build
c++ -std=c++17 -Wall -Wextra -Iinclude tests/update_version_test.cpp -o tests/build/update_version_test
tests/build/update_version_test
c++ -std=c++17 -Wall -Wextra -pthread -Iinclude tests/update_test.cpp source/update/updater.cpp -lminizip -lz -Wl,--wrap=rename -o tests/build/update_test
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT INT TERM
python3 tests/update_fixture.py "$fixture"
tests/build/update_test "$fixture"
