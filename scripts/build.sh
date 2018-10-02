#!/usr/bin/env bash
set -e

version="{\"Major\":0,\"Minor\":0,\"Build\":\"0+dev.0\"}"

WORKSPACE="$PWD"

mkdir -p $WORKSPACE/build_artifacts
pushd "$GOPATH/src/code.cloudfoundry.org/log-cache-cli/cmd/cf-lc-plugin"
  GOOS=linux go build -ldflags "-X main.version=$version" -o $WORKSPACE/build_artifacts/log-cache-cf-plugin-linux
popd
pushd "$GOPATH/src/code.cloudfoundry.org/log-cache-cli/cmd/lc"
  GOOS=linux go build -ldflags "-X main.version=$version" -o $WORKSPACE/build_artifacts/log-cache-linux
popd
