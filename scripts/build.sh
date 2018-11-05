#!/usr/bin/env bash
set -e

version="{\"Major\":0,\"Minor\":0,\"Build\":\"0+dev.0\"}"

SCRIPTS_PATH="$( cd "$(dirname "$0")" ; pwd -P )"
WORKSPACE="$SCRIPTS_PATH/.."

pushd $WORKSPACE
  go get ./...
popd

pushd "$WORKSPACE/cmd/cf-lc-plugin"
  go build -ldflags "-X main.version=$version" -o $WORKSPACE/build_artifacts/log-cache-cf-plugin-dev
popd

pushd "$WORKSPACE/cmd/lc"
  go build -ldflags "-X main.version=$version" -o $WORKSPACE/build_artifacts/log-cache-standalone-dev
popd
