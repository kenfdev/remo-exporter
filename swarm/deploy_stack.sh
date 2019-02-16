#!/bin/sh

BASEDIR=$(dirname "$0")
COMPOSE_FILE="${BASEDIR}/../docker-compose.yml"

docker stack deploy func --compose-file ${COMPOSE_FILE}
