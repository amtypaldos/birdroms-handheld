#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
docker build -f tests/Dockerfile -t birdroms-switch-tests .
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work birdroms-switch-tests
