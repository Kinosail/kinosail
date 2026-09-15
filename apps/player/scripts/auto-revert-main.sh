#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

remote="${1:-origin}"
branch="${2:-main}"
expected="${3:?expected failing commit SHA is required}"
git fetch "$remote" "$branch"
tip="$(git rev-parse "$remote/$branch")"
if [[ "$tip" != "$expected" ]]; then
  printf 'SKIP auto-revert: %s advanced to %s\n' "$branch" "$tip"
  exit 0
fi
subject="$(git show -s --format=%s "$tip")"
if [[ "$subject" == *'[auto-revert]'* ]]; then
  printf 'SKIP auto-revert: tip is already an automatic revert\n'
  exit 0
fi
if [[ "$(git show -s --format=%P "$tip" | wc -w | tr -d ' ')" != 1 ]]; then
  printf 'SKIP auto-revert: tip is not a single-parent commit\n'
  exit 0
fi
git checkout --detach "$tip"
git config user.name "Kinosail CI"
git config user.email "actions@users.noreply.github.com"
git revert --no-edit "$tip"
git commit --amend -m "Revert \"$subject\" [auto-revert]"
git push "$remote" HEAD:"$branch"
printf 'REVERTED %s\n' "$tip"
