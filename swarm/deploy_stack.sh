#!/bin/sh

set -eu

BASEDIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

exec docker stack deploy func --compose-file "${BASEDIR}/swarm/docker-compose.yml"
