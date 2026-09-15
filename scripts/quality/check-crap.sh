#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-metrics.XXXXXX")"
trap 'rm -rf "$tools"' EXIT
GOWORK=off go -C "$repo/scripts/quality/metrics" build -o "$tools/metrics" .
for app in player subtitles dashboard packages; do
	targets=(./...)
	if [[ "$app" == packages ]]; then
		cd "$repo/packages"
		targets=()
		while IFS= read -r package; do
			case "$package" in
			*/archivetest|*/commandtest|*/configurationtest|*/servertest) ;;
			*) targets+=("$package") ;;
			esac
		done < <(go list ./...)
	else
		cd "$repo/apps/$app"
	fi
	mkdir -p .verification
	if [[ ! -f .verification/coverage.out ]]; then
		if [[ -x scripts/with-go-module.sh ]]; then
			./scripts/with-go-module.sh go test -count=1 -coverprofile=.verification/coverage.out ./...
		else
			go test -count=1 -coverprofile=.verification/coverage.out ./...
		fi
	fi
	"$tools/metrics" crap .verification/coverage.out "${targets[@]}"
done
