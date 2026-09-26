#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="${TMPDIR:-/tmp}/kinosail-quality-tools"
mkdir -p "$tools"
GOBIN="$tools" go install golang.org/x/tools/cmd/deadcode@v0.49.1-0.20260903194427-2b98ca46aac4
status=0
targets=("$@")
if (( ${#targets[@]} == 0 )); then
	targets=(player subtitles packages)
fi

for target in "${targets[@]}"; do
	case "$target" in
	player|subtitles)
		module="$(awk '$1 == "module" { print $2; exit }' "$repo/apps/$target/go.mod")"
		output="$(cd "$repo/apps/$target" && "$tools/deadcode" -filter="$module" ./...)"
		;;
	packages)
		module="$(awk '$1 == "module" { print $2; exit }' "$repo/packages/go.mod")"
		output="$(cd "$repo" && "$tools/deadcode" -filter="$module" ./apps/player/... ./apps/subtitles/... ./packages/... | sed -E '\#^packages/(archivetest|commandtest|configurationtest|servertest)/#d')"
		test_output="$(cd "$repo" && "$tools/deadcode" -test -filter="^$module/(archivetest|commandtest|configurationtest|servertest)$" ./apps/player/... ./apps/subtitles/... ./packages/...)"
		if [[ -n "$test_output" ]]; then
			output+="${output:+$'\n'}$test_output"
		fi
		;;
	*) printf 'unknown dead-code target: %s\n' "$target" >&2; exit 2 ;;
	esac
	if [[ -n "$output" ]]; then
		printf '%s\n' "$output" >&2
		status=1
	fi
done

exit "$status"
