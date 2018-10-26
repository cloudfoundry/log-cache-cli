#!/usr/bin/env bash

set -e

git submodule update --init --recursive --rebase

SCRIPTS_PATH="$( cd "$(dirname "$0")" ; pwd -P )"
WORKSPACE="$SCRIPTS_PATH/.."

$SCRIPTS_PATH/build.sh

# Install the log-cache plugin to the CF CLI and force overwrite
cf install-plugin $WORKSPACE/build_artifacts/log-cache-cf-plugin-dev -f

# Install the standalone log-cache CLI for k8s
sudo cp $WORKSPACE/build_artifacts/log-cache-standalone-dev /usr/local/bin/lc

rm -rf $WORKSPACE/build_artifacts
