#!/usr/bin/env bash

set -e

cf uninstall-plugin log-cache || true # suppress errors

git submodule update --init --recursive --rebase
./scripts/build.sh

cf install-plugin ./build_artifacts/log-cache-cf-plugin-linux -f
rm -rf ./build_artifacts
