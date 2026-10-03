#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
mkdir -p dist

for arch in amd64 arm64 arm; do
	suffix=$arch
	if [ "$arch" = arm ]; then
		suffix=armv7
	fi
	GOOS=linux GOARCH=$arch GOARM=7 CGO_ENABLED=0 \
		go build -trimpath "$@" -o "dist/remo-exporter-linux-$suffix" .
done
