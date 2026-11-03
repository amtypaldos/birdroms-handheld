#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p tests/build
c++ -std=c++17 -Wall -Wextra -pthread -Iinclude ${CURL_CFLAGS:-} source/api/client.cpp tests/update_http_test.cpp ${CURL_LIBS:--lcurl} -o tests/build/update_http_test
fixture=$(mktemp -d)
python3 tests/update_fixture.py "$fixture"
python3 tests/update_http_fixture.py "$fixture" &
server=$!
trap 'kill "$server" 2>/dev/null || true; rm -rf "$fixture"' EXIT INT TERM
while [ ! -s "$fixture/http_port" ]; do sleep .05; done
tests/build/update_http_test "http://127.0.0.1:$(cat "$fixture/http_port")"
