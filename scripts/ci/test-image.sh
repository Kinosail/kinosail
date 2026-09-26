#!/usr/bin/env bash
# Validate publication inputs before pulling or starting anything.
set -euo pipefail
[[ $# == 2 ]] || { echo 'expected app and immutable image reference' >&2; exit 2; }
case "$1" in player|subtitles) ;; *) echo 'invalid app' >&2; exit 2 ;; esac
[[ "$2" =~ ^ghcr\.io/kinosail/kinosail-$1@sha256:[a-f0-9]{64}$ ]] || { echo 'invalid image digest reference' >&2; exit 2; }
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
docker pull "$2"
cd "$repo/apps/$1"
CONTAINER_ENGINE=docker KINOSAIL_TEST_IMAGE="$2" KINOSAIL_TEST_IMAGE_READY=1 ./scripts/test-container.sh
