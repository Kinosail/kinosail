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
  Movies/Example\ Movie.mp4 \
  Shows/Example\ Show/Season\ 01/Example\ Show\ S01E01\ Example\ Episode\ One.mp4 \
  Music/Example\ Album/Example\ Track\ One.m4a \
  Audiobooks/Example\ Audiobook.m4b \
  Books/Example\ PDF\ Book.pdf \
  Photos/Geometry/Example\ Photo\ One.jpg \
  TMDB/api/search/movie \
  CC0-1.0.txt; do
  [[ -s "$test_root/media/$path" ]]
done
