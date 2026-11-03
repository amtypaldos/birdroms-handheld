#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p tests/build
c++ -std=c++17 -Wall -Wextra -Iinclude tests/api_test.cpp -o tests/build/api_test
tests/build/api_test
# Host curl headers may be supplied through CURL_CFLAGS when using the devkitPro container.
c++ -std=c++17 -Wall -Wextra -pthread -Iinclude ${CURL_CFLAGS:-} source/api/client.cpp tests/api_integration.cpp ${CURL_LIBS:--lcurl} -o tests/build/api_integration
rm -f tests/build/api_port
python3 tests/api_fixture.py tests/build/api_port &
fixture=$!
trap 'kill "$fixture" 2>/dev/null || true' EXIT INT TERM
while [ ! -s tests/build/api_port ]; do sleep .05; done
tests/build/api_integration "http://127.0.0.1:$(cat tests/build/api_port)"
