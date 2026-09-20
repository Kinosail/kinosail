#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

if (( $# > 1 )); then
  echo 'expected at most one scope: player, subtitles, dashboard, packages' >&2
  exit 2
fi
if (( $# == 1 )); then
  case "$1" in player|subtitles|dashboard|packages) ;; *) echo 'invalid quality scope' >&2; exit 2 ;; esac
fi

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-metrics.XXXXXX")"
trap 'rm -rf "$tools"' EXIT
GOWORK=off go -C "$repo/scripts/quality/metrics" build -o "$tools/metrics" .
apps=(player subtitles dashboard packages)
if (( $# == 1 )); then apps=("$1"); fi
for app in "${apps[@]}"; do
	targets=(./...)
	if [[ "$app" == packages ]]; then
		cd "$repo/packages"
		targets=()
		while IFS= read -r package; do
			case "$package" in
			*/archivetest|*/archivetest/*|*/commandtest|*/commandtest/*|*/configurationtest|*/configurationtest/*|*/servertest|*/servertest/*) ;;
			*) targets+=("$package") ;;
			esac
		done < <(go list ./...)
	else
		cd "$repo/apps/$app"
	fi
	mkdir -p .verification
	if [[ ! -f .verification/coverage.out ]]; then
		if [[ -x scripts/with-go-module.sh ]]; then
			./scripts/with-go-module.sh go test -count=1 -coverprofile=.verification/coverage.out "${targets[@]}"
		else
			go test -count=1 -coverprofile=.verification/coverage.out "${targets[@]}"
		fi
	fi
	"$tools/metrics" crap .verification/coverage.out "${targets[@]}"
done
