#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if (( $# < 1 || $# > 2 )); then
  printf 'usage: %s {apps/player|apps/subtitles} [base-revision]\n' "$0" >&2
  exit 2
fi
app_path="$1"
case "$app_path" in apps/player|apps/subtitles) ;; *) printf 'unknown verification app: %s\n' "$app_path" >&2; exit 2 ;; esac
app="$repo/$app_path"
base="${2:-origin/main}"
previous=""
cd "$app"
printf 'Watching tracked changes; press Ctrl-C to stop.\n'
while true; do
  current="$( {
    git -C "$repo" status --porcelain=v1 --untracked-files=all -- "$app_path" packages go.work go.work.sum
    git -C "$repo" diff "$base"...HEAD -- "$app_path" packages go.work go.work.sum
  } | shasum -a 256 | awk '{print $1}')"
  if [[ "$current" != "$previous" ]]; then
    previous="$current"
    KINOSAIL_VERIFY_WORKTREE=1 "$repo/scripts/tooling/verify-changed.sh" "$app_path" "$base" || true
  fi
  sleep 2
done
