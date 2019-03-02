#!/bin/sh

_remo_exporter_tag=$1
_docker_repo=${2:-kenfdev/remo-exporter}

# If the tag starts with v, treat this as a official release
if echo "$_remo_exporter_tag" | grep -q "^v"; then
	_remo_exporter_version=$(echo "${_remo_exporter_tag}" | cut -d "v" -f 2)
else
	_remo_exporter_version=$_remo_exporter_tag
fi

echo "Building ${_docker_repo}:${_remo_exporter_version}"

export DOCKER_CLI_EXPERIMENTAL=enabled

# Build remo-exporter image for a specific arch
docker_build () {
	base_image=$1
	exporter_binary=$2
	tag=$3

  docker build \
		--build-arg BASE_IMAGE=${base_image} \
		--build-arg EXPORTER_BINARY=${exporter_binary} \
		--tag "${tag}" \
		--no-cache=true .
}

# Tag docker images of all architectures
docker_tag_all () {
	repo=$1
	tag=$2
	docker tag "${_docker_repo}:${_remo_exporter_version}" "${repo}:${tag}"
	docker tag "${_docker_repo}-linux-arm32v7:${_remo_exporter_version}" "${repo}-linux-arm32v7:${tag}"
	docker tag "${_docker_repo}-linux-arm64v8:${_remo_exporter_version}" "${repo}-linux-arm64v8:${tag}"
}

docker_build "alpine:3.9" "remo-exporter-linux-amd64" "${_docker_repo}:${_remo_exporter_version}"
docker_build "arm32v6/alpine:3.9" "remo-exporter-linux-armv7" "${_docker_repo}-linux-arm32v7:${_remo_exporter_version}"
docker_build "arm64v8/alpine:3.9" "remo-exporter-linux-amd64" "${_docker_repo}-linux-arm64v8:${_remo_exporter_version}"

# Tag as 'latest' for official release; otherwise tag as kenfdev/remo-exporter:master
if echo "$_remo_exporter_tag" | grep -q "^v"; then
	docker_tag_all "${_docker_repo}" "latest"
else
	docker_tag_all "${_docker_repo}" "master"
	docker tag "${_docker_repo}:${_remo_exporter_version}" "kenfdev/remo-exporter-dev:${_remo_exporter_version}"
fi
