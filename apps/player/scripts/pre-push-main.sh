#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
app="$repo/apps/player"
[[ -f "$app/Makefile" ]] || { printf 'Kinosail Player app not found: %s\n' "$app" >&2; exit 2; }
cd "$app"

# Keep the universal LOC boundary, then run only checks selected by the main diff.
main_push=0
while read -r _ _ remote_ref remote_sha; do
  [[ "$remote_ref" == refs/heads/main ]] || continue
  main_push=1
  if [[ "$remote_sha" =~ ^0+$ ]] || ! git cat-file -e "$remote_sha^{commit}" 2>/dev/null; then
    remote_sha=origin/main
  fi
  ./scripts/verify-changed.sh "$remote_sha"
done
(( main_push == 1 )) || make max-loc
