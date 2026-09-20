#!/usr/bin/env bash
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
common="$(git rev-parse --path-format=absolute --git-common-dir)"
python3 "$repo/scripts/tooling/check-file-loc.py" --staged
if [[ -f "$repo/.gates-disabled" || -f "$common/../.gates-disabled" ]]; then
  printf "Automatic quality gates are paused (.gates-disabled).\n"
  exit 0
fi
"$repo/scripts/tooling/worktree_guard.py" heartbeat --if-present
"$repo/scripts/tooling/worktree_guard.py" audit
printf 'Full quality suites run in GitHub Actions.\n'
