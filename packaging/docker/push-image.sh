#!/bin/sh
set -eu

if [ "${1:-}" != --publish ] || [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
	echo "Publication is disabled by default. Usage: $0 --publish TAG [REPOSITORY]" >&2
	exit 2
fi
if [ -n "${CI:-}${CIRCLECI:-}${GITHUB_ACTIONS:-}" ]; then
	echo 'Image publication is disabled in CI.' >&2
	exit 1
fi

tag=$2
repo=${3:-kenfdev/remo-exporter}
. "$(dirname -- "$0")/release-channel.sh"

docker_push_all() {
	publish_tag=$1
	for suffix in '' -linux-arm32v7 -linux-arm64v8; do
		docker push "$repo$suffix:$publish_tag"
	done
	docker manifest create --amend "$repo:$publish_tag" \
		"$repo:$publish_tag" \
		"$repo-linux-arm32v7:$publish_tag" \
		"$repo-linux-arm64v8:$publish_tag"
	docker manifest annotate "$repo:$publish_tag" "$repo-linux-arm32v7:$publish_tag" \
		--os linux --arch arm --variant v7
	docker manifest push --purge "$repo:$publish_tag"
}

docker_push_all "$version"
case "$channel:$tag" in
	stable:*) docker_push_all latest ;;
	development:master*)
		docker_push_all master
		docker push "$repo-dev:$version"
		;;
esac
