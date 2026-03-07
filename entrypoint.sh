#!/bin/bash
set -eo pipefail

DEFAULT_APP=${DEFAULT_APP:-"/app/bin/zaplab"}

# Ensure the data directory and required subdirectories exist.
ZAPLAB_DATA_DIR=${ZAPLAB_DATA_DIR:-"${HOME}/.zaplab"}
mkdir -p \
    "${ZAPLAB_DATA_DIR}/pb_data" \
    "${ZAPLAB_DATA_DIR}/db" \
    "${ZAPLAB_DATA_DIR}/history" \
    "${ZAPLAB_DATA_DIR}/logs" \
    "${ZAPLAB_DATA_DIR}/n8n"

# Prepend the app binary when the first argument is a subcommand or flag.
if [ "${1:0:1}" = '-' ] || [ "$1" = "serve" ] || [ "$1" = "admin" ] || [ "$1" = "migrate" ] || [ "$1" = "update" ]; then
    set -- "$DEFAULT_APP" "$@"
fi

exec "$@"
