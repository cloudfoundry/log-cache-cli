#!/usr/bin/env bash

set -e

version="{\"Major\":0,\"Minor\":0,\"Build\":\"0+dev.0\"}"

SCRIPTS_PATH="$( cd "$(dirname "$0")" ; pwd -P )"
WORKSPACE="$SCRIPTS_PATH/.."

pushd $WORKSPACE
  go get ./...
  go build -ldflags "-X main.version=$version" -o $WORKSPACE/bin/log-cache-cf-plugin-dev
popd
