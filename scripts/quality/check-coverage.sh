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
status=0

apps=(player subtitles dashboard)
if (( $# == 1 )); then
  apps=("$1")
fi
for app in "${apps[@]}"; do
	[[ "$app" != packages ]] || continue
	directory="$repo/apps/$app"
	mkdir -p "$directory/.verification"
	if [[ -x "$directory/scripts/with-go-module.sh" ]]; then
		if ! (cd "$directory" && ./scripts/with-go-module.sh go test -count=1 -coverprofile=.verification/coverage.out ./...); then
			status=1
		fi
	else
		if ! (cd "$directory" && go test -count=1 -coverprofile=.verification/coverage.out ./...); then
			status=1
		fi
	fi
	if [[ ! -s "$directory/.verification/coverage.out" ]]; then
		printf '%s coverage profile was not produced\n' "$app" >&2
		status=1
		continue
	fi
	coverage="$(cd "$directory" && go tool cover -func=.verification/coverage.out | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
	minimum=100.0
	case "$app" in
		player|subtitles) minimum=89.0 ;;
	esac
	if ! awk -v app="$app" -v coverage="$coverage" -v minimum="$minimum" 'BEGIN {
		if (coverage + 0 < minimum) {
			printf "%s coverage %.1f%% is below %.1f%%\n", app, coverage, minimum
			exit 1
		}
	}'; then
		status=1
	fi
done

if (( $# == 1 )) && [[ "$1" != packages ]]; then exit "$status"; fi

mkdir -p "$repo/packages/.verification"
package_targets=()
while IFS= read -r package; do
	case "$package" in
	*/archivetest|*/commandtest|*/configurationtest|*/servertest) ;;
	*) package_targets+=("$package") ;;
	esac
done < <(cd "$repo/packages" && go list ./...)
if ! (cd "$repo/packages" && go test -count=1 -coverprofile=.verification/coverage.out "${package_targets[@]}"); then
	status=1
fi
if [[ ! -s "$repo/packages/.verification/coverage.out" ]]; then
	printf 'packages coverage profile was not produced\n' >&2
	status=1
else
	packages_coverage="$(cd "$repo/packages" && go tool cover -func=.verification/coverage.out | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
	if ! awk -v coverage="$packages_coverage" 'BEGIN {
		if (coverage + 0 != 100) {
			printf "packages coverage %.1f%% is below 100.0%%\n", coverage
			exit 1
		}
	}'; then
		status=1
	fi
fi

exit "$status"
