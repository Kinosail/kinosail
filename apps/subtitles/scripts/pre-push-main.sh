#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
app="$repo/apps/subtitles"
[[ -f "$app/Makefile" ]] || { printf 'Kinosail Subtitles app not found: %s\n' "$app" >&2; exit 2; }
cd "$app"

# Keep the universal LOC boundary, then run only checks selected by the main diff.
main_push=0
while read -r _ _ remote_ref remote_sha; do
  [[ "$remote_ref" == refs/heads/main ]] || continue
  main_push=1
  if [[ "$remote_sha" =~ ^0+$ ]] || ! git cat-file -e "$remote_sha^{commit}" 2>/dev/null; then
    if git rev-parse --verify 'origin/main^{commit}' >/dev/null 2>&1; then
      remote_sha=origin/main
    elif git rev-parse --verify 'refs/kinosail/template^{commit}' >/dev/null 2>&1; then
      remote_sha=refs/kinosail/template
    else
      remote_sha="$(git rev-list --max-parents=0 HEAD | tail -1)"
    fi
  fi
  ./scripts/verify-changed.sh "$remote_sha"
done
(( main_push == 1 )) || make max-loc
