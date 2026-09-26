#!/usr/bin/env bash
# Execute each affected module once, including races, then inspect that profile.
set -euo pipefail
[[ $# == 1 ]] || { echo 'expected one Go module' >&2; exit 2; }
case "$1" in
  player|subtitles) directory="apps/$1"; minimum=89 ;;
  packages) directory=packages; minimum=85 ;;
  *) echo 'invalid Go module' >&2; exit 2 ;;
esac
mode="${KINOSAIL_GO_TEST_MODE:-deep}"
case "$mode" in quick|deep) ;; *) echo 'invalid Go test mode' >&2; exit 2 ;; esac
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo/$directory"
if [[ "$mode" == quick ]]; then
  go test -count=1 ./...
  exit
fi
mkdir -p .verification
profile=.verification/coverage.out
rm -f "$profile"
go test -race -count=1 -covermode=atomic -coverprofile="$profile" ./...
# Test-support packages are exercised by consumers, not standalone coverage.
if [[ "$1" == packages ]]; then
  awk 'NR == 1 || $1 !~ /\/(archivetest|commandtest|configurationtest|servertest)\//' "$profile" > "$profile.filtered"
  mv "$profile.filtered" "$profile"
fi
coverage="$(go tool cover -func="$profile" | awk '/^total:/ {sub(/%$/, "", $3); print $3}')"
awk -v coverage="$coverage" -v minimum="$minimum" 'BEGIN {
  if (coverage !~ /^[0-9]+([.][0-9]+)?$/ || coverage + 0 < minimum) {
    printf "coverage %s%% is below %s%%\n", coverage, minimum; exit 1
  }
  printf "race suite passed; coverage %s%% (minimum %s%%)\n", coverage, minimum
}'
