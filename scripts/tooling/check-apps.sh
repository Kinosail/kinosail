#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
apps=(player subtitles)

if (( $# > 0 )); then
  apps=("$@")
fi

for app in "${apps[@]}"; do
  case "$app" in
    player|subtitles) ;;
    *)
      printf 'Unknown app: %s\n' "$app" >&2
      exit 2
      ;;
  esac
done

printf '\n==> Checking shared packages\n'
make -C "$repo/packages" check

for app in "${apps[@]}"; do
  printf '\n==> Checking %s\n' "$app"
  make -C "$repo/apps/$app" check
done
