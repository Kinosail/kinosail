#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
export KINOSAIL_TEST_ROOT="$repo/.kinosail-test/playback"
export KINOSAIL_TEST_PROJECT=kinosail-playback-test
export KINOSAIL_PORT=0

cleanup() {
  ./scripts/test-instance.sh down --volumes >/dev/null 2>&1 || true
}
trap cleanup EXIT

./scripts/test-instance.sh up
./scripts/test-instance.sh playback
printf 'Playback artifacts: %s\n' "$KINOSAIL_TEST_ROOT/playback-results"
