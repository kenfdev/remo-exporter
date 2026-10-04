#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
	echo "Usage: $0 TAG [REPOSITORY]" >&2
	exit 2
fi

tag=$1
repo=${2:-kenfdev/remo-exporter}
. "$(dirname -- "$0")/release-channel.sh"

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_dir"

while read -r platform binary suffix; do
	docker build \
		--platform "$platform" \
		--build-arg "EXPORTER_BINARY=remo-exporter-linux-$binary" \
		--tag "$repo$suffix:$version" \
		--file packaging/docker/Dockerfile dist
done <<'ARCHITECTURES'
linux/amd64 amd64
linux/arm/v7 armv7 -linux-arm32v7
linux/arm64 arm64 -linux-arm64v8
ARCHITECTURES

case "$channel" in
	stable) alias=latest ;;
	development) alias=master ;;
	*) exit 0 ;;
esac

for suffix in '' -linux-arm32v7 -linux-arm64v8; do
	docker tag "$repo$suffix:$version" "$repo$suffix:$alias"
done

case "$channel" in
	development) docker tag "$repo:$version" "$repo-dev:$version" ;;
esac
