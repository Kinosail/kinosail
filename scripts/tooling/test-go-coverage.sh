#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

if (( $# != 2 )); then
  printf 'usage: %s {packages|apps/player|apps/subtitles} minimum-percent\n' "$0" >&2
  exit 2
fi
scope="$1"
minimum="$2"
case "$scope" in
  packages|apps/player|apps/subtitles) ;;
  *) printf 'unknown Go coverage scope: %s\n' "$scope" >&2; exit 2 ;;
esac
[[ "$minimum" =~ ^([0-9]{1,2}|100)(\.[0-9]+)?$ ]] || { printf 'invalid minimum coverage: %s\n' "$minimum" >&2; exit 2; }
awk -v minimum="$minimum" 'BEGIN { exit !(minimum <= 100) }' || { printf 'invalid minimum coverage: %s\n' "$minimum" >&2; exit 2; }

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
coverage_file="$(mktemp "${TMPDIR:-/tmp}/kinosail-coverage.XXXXXX")"
trap 'rm -f -- "$coverage_file"' EXIT
cd "$repo/$scope"
targets=(./...)
if [[ "$scope" == packages ]]; then
	targets=()
	while IFS= read -r package; do
		case "$package" in
		*/archivetest|*/archivetest/*|*/commandtest|*/commandtest/*|*/configurationtest|*/configurationtest/*|*/servertest|*/servertest/*) ;;
		*) targets+=("$package") ;;
		esac
	done < <(go list ./...)
fi
"$repo/scripts/tooling/with-go-module.sh" go test -count=1 -coverprofile="$coverage_file" "${targets[@]}"
coverage="$("$repo/scripts/tooling/with-go-module.sh" go tool cover -func="$coverage_file" | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
awk -v coverage="$coverage" -v minimum="$minimum" 'BEGIN { if (coverage + 0 < minimum + 0) { printf "coverage %.1f%% is below %.1f%%\n", coverage, minimum; exit 1 } }'
printf 'total coverage: %s%% (minimum %s%%)\n' "$coverage" "$minimum"
