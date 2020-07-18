#!/bin/bash

set -e

EXTRA_OPTS="$@"

CCARMV7=arm-linux-gnueabihf-gcc
CCARM64=aarch64-linux-gnu-gcc

GOPATH=~/go
REPO_PATH=$GOPATH/src/github.com/kenfdev/remo-exporter

cd ~/go/src/github.com/kenfdev/remo-exporter
echo "current dir: $(pwd)"

echo "Build arguments: $OPT"

export GOOS=linux
export CGO_ENABLED=0

# TODO: -ldflags

export GOARCH=arm
export GOARM=7
CC=${CCARMV7} go build -o ./dist/remo-exporter-${GOOS}-${GOARCH}v7

export GOARCH=arm64
CC=${CCARM64} go build -o ./dist/remo-exporter-${GOOS}-${GOARCH}

export GOARCH=amd64
go build -o ./dist/remo-exporter-${GOOS}-${GOARCH}
