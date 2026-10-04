#!/usr/bin/env bash
# Warm the shared module cache without rewriting the proof checkout.
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
# Original graph and checksum verification remain enabled. Scratch is retained
# until the disposable hosted runner ends; it is never copied over source.
GOWORK="$scratch/go.work" go -C "$scratch/apps/$app" mod download
