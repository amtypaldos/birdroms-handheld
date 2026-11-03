#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
docker build -t birdroms-switch-toolchain .
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work birdroms-switch-toolchain make "$@"
