#!/bin/sh
set -eu

if [ -n "${CIRCLECI:-}" ]; then
	echo "Tests moved to GitHub Actions. Refusing the legacy CircleCI upload pipeline." >&2
	exit 1
fi

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
go test -race -coverprofile=coverage.txt -covermode=atomic ./...
