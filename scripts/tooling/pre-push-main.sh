#!/usr/bin/env bash
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
# Nested Git fixtures must discover their own repositories, not inherit this hook's context.
while IFS= read -r git_context; do
  unset "$git_context"
done < <(git rev-parse --local-env-vars)
cd "$repo"
payload="$(mktemp "${TMPDIR:-/tmp}/kinosail-pre-push.XXXXXX")"
trap 'rm -f "$payload"' EXIT
cat >"$payload"

while read -r _ local_sha _ _; do
  [[ "$local_sha" =~ ^0+$ ]] && continue
  python3 "$repo/scripts/tooling/check-file-loc.py" --revision "$local_sha"
done <"$payload"
if [[ -f "$repo/.gates-disabled" || -f "$common/../.gates-disabled" ]]; then
  printf "Automatic quality gates are paused (.gates-disabled).\n"
  exit 0
fi
"$repo/scripts/tooling/worktree_guard.py" heartbeat --if-present

printf 'Full quality suites run in GitHub Actions.\n'
