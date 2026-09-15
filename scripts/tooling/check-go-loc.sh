#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
scope="${1:-}"
limit="${2:-300}"

case "$scope" in
  packages|apps/player|apps/subtitles|apps/dashboard) ;;
  *) printf 'unknown Go source scope: %s\n' "$scope" >&2; exit 2 ;;
esac
[[ "$limit" =~ ^[1-9][0-9]{0,3}$ ]] || { printf 'invalid line limit: %s\n' "$limit" >&2; exit 2; }

failed=0
cd "$repo/$scope"
while IFS= read -r -d '' file; do
  [[ -f "$file" ]] || continue
  if head -n 20 "$file" | grep -Eq '^// Code generated .* DO NOT EDIT\.$'; then
    continue
  fi
  lines="$(wc -l < "$file" | tr -d ' ')"
  if (( lines > limit )); then
    printf '%s: %d lines (maximum %d)\n' "$file" "$lines" "$limit" >&2
    failed=1
  fi
done < <(git ls-files -co --exclude-standard -z -- '*.go')

if (( failed )); then
  printf 'Split oversized Go files by responsibility.\n' >&2
  exit 1
fi
