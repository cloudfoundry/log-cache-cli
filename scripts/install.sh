#!/usr/bin/env bash

set -e

SCRIPTS_PATH="$( cd "$(dirname "$0")" ; pwd -P )"
WORKSPACE="$SCRIPTS_PATH/.."

$SCRIPTS_PATH/build.sh

# Install the log-cache plugin to the CF CLI and force overwrite
cf install-plugin $WORKSPACE/bin/log-cache-cf-plugin-dev -f

rm -rf $WORKSPACE/bin
