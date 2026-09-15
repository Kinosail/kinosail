#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$repo/.kinosail-test"
test_root="$(mktemp -d "$repo/.kinosail-test/run.XXXXXX")"
export KINOSAIL_TEST_ROOT="$test_root"
export KINOSAIL_TEST_PROJECT="kinosail-test-$$-$RANDOM"
export KINOSAIL_PORT=0

cleanup() {
  ./scripts/test-instance.sh down --volumes >/dev/null 2>&1 || true
  rm -rf -- "${test_root:?}"
}
trap cleanup EXIT

./scripts/test-instance.sh up
./scripts/test-instance.sh verify
./scripts/test-instance.sh browser

for path in \
  Movies/CC0\ Provider\ Candidate.mp4 \
  Shows/Signal\ Garden/Season\ 01/Signal\ Garden\ S01E01\ First\ Light.mp4 \
  Music/Signal\ Lab/Synthetic\ Pulse.m4a \
  Audiobooks/The\ Geometry\ of\ Sound.m4b \
  Books/The\ Test\ Voyage.pdf \
  Photos/Geometry/Color\ Study.jpg \
  TMDB/api/search/movie \
  CC0-1.0.txt; do
  [[ -s "$test_root/media/$path" ]]
done
