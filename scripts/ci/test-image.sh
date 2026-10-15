#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$repo/scripts/ci/pull-image.sh" "$@"
cd "$repo/apps/$1"
CONTAINER_ENGINE=docker KINOSAIL_TEST_IMAGE="$2" KINOSAIL_TEST_IMAGE_READY=1 ./scripts/test-container.sh
