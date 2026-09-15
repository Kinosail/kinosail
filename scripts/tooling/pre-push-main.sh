#!/usr/bin/env bash
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
if [[ -f "$repo/.gates-disabled" || -f "$common/../.gates-disabled" ]]; then
  printf "Automatic quality gates are paused (.gates-disabled).\n"
  exit 0
fi
# Nested Git fixtures must discover their own repositories, not inherit this hook's context.
while IFS= read -r git_context; do
  unset "$git_context"
done < <(git rev-parse --local-env-vars)
cd "$repo"
"$repo/scripts/tooling/worktree_guard.py" heartbeat --if-present
payload="$(mktemp "${TMPDIR:-/tmp}/kinosail-pre-push.XXXXXX")"
trap 'rm -f "$payload"' EXIT
cat >"$payload"

lint_base=HEAD
while read -r _ _ remote_ref remote_sha; do
  [[ "$remote_ref" == refs/heads/main ]] || continue
  if [[ "$remote_sha" =~ ^0+$ ]] || ! git cat-file -e "$remote_sha^{commit}" 2>/dev/null; then
    remote_sha=origin/main
  fi
  lint_base="$remote_sha"
done <"$payload"

export KINOSAIL_MUTATION_DIFF="$lint_base"
make -C "$repo" tooling-check
make -C "$repo/packages" check
KINOSAIL_LINT_BASE="$lint_base" "$repo/scripts/quality/check-full.sh"

for app in player subtitles dashboard; do
  "$repo/apps/$app/scripts/pre-push-main.sh" "$@" <"$payload"
done
