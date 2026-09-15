#!/usr/bin/env bash
set -euo pipefail

scripts="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$scripts/.." && pwd)"
script="$scripts/browser-projects.sh"

assert_projects() {
  local expected="$1"
  shift
  local actual
  actual="$(env -u KINOSAIL_BROWSER_MATRIX -u KINOSAIL_BROWSER_PROJECT "$@" "$script")"
  [[ "$actual" == "$expected" ]]
}

assert_projects chromium
assert_projects $'chromium\nfirefox\nwebkit' env KINOSAIL_BROWSER_MATRIX=full
assert_projects firefox env KINOSAIL_BROWSER_MATRIX=full KINOSAIL_BROWSER_PROJECT=firefox
if env KINOSAIL_BROWSER_PROJECT=opera "$script" >/dev/null 2>&1; then
  printf 'unsupported browser project was accepted\n' >&2
  exit 1
fi
if env KINOSAIL_BROWSER_MATRIX=wide "$script" >/dev/null 2>&1; then
  printf 'unsupported browser matrix was accepted\n' >&2
  exit 1
fi

projects="$(cd "$repo" && KINOSAIL_BROWSER_PROJECT=firefox pnpm --dir e2e exec playwright test --list conditional-states.spec.ts)"
grep --quiet '\[firefox\]' <<<"$projects"
if grep --quiet '\[chromium\]\|\[webkit\]' <<<"$projects"; then
  printf 'selected Firefox project included another browser\n' >&2
  exit 1
fi
