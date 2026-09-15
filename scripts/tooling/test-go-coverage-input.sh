#!/usr/bin/env bash
# shellcheck disable=SC2016 # The quoted variable belongs to the generated Go mock.
set -euo pipefail

tool="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/test-go-coverage.sh"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-coverage-input-test.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin"
printf '%s\n' '#!/usr/bin/env bash' ': >"$KINOSAIL_TEST_SIDE_EFFECT"' 'exit 99' >"$tmp/bin/go"
chmod +x "$tmp/bin/go"

reject() {
  if PATH="$tmp/bin:$PATH" KINOSAIL_TEST_SIDE_EFFECT="$tmp/side-effect" "$tool" "$@" >/dev/null 2>&1; then
    printf 'invalid coverage input was accepted: %s\n' "$*" >&2
    exit 1
  fi
  [[ ! -e "$tmp/side-effect" ]] || { printf 'invalid coverage input caused a side effect: %s\n' "$*" >&2; exit 1; }
}

reject
reject unknown 80
reject ../apps/player 80
reject packages
reject packages invalid
reject packages -1
reject packages 101
reject packages 100.1

printf 'Go coverage input tests passed\n'
