#!/usr/bin/env bash
set -euo pipefail
[[ $# == 1 ]] || exit 2
case "$1" in player|subtitles) app="$1" ;; *) exit 2 ;; esac
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
scratch="$(mktemp -d "${RUNNER_TEMP:?}/campaign-go.XXXXXX")"
for file in go.work go.work.sum apps/player/go.mod apps/player/go.sum \
  apps/subtitles/go.mod apps/subtitles/go.sum packages/go.mod packages/go.sum; do
  [[ -f "$repo/$file" && ! -L "$repo/$file" ]] || exit 2
  mkdir -p "$scratch/$(dirname "$file")"
  cp "$repo/$file" "$scratch/$file"
done
GOWORK="$scratch/go.work" go -C "$scratch/apps/$app" mod download
