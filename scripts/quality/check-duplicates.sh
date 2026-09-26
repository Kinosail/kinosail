#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="${TMPDIR:-/tmp}/kinosail-quality-tools"
mkdir -p "$tools"

base="${KINOSAIL_LINT_BASE:-}"
if [[ -z "$base" ]]; then
	if upstream="$(git -C "$repo" rev-parse --verify origin/main 2>/dev/null)" && [[ "$upstream" != "$(git -C "$repo" rev-parse HEAD)" ]]; then
		base="$(git -C "$repo" merge-base HEAD "$upstream")"
	else
		base="$(git -C "$repo" rev-parse HEAD^ 2>/dev/null || git -C "$repo" rev-list --max-parents=0 HEAD | tail -1)"
	fi
fi
changed="$(mktemp "${TMPDIR:-/tmp}/kinosail-duplicate-files.XXXXXX")"
trap 'rm -f "$changed"' EXIT
git -C "$repo" diff --name-only --diff-filter=ACMRT "$base"...HEAD -- '*.go' >"$changed"
if [[ ! -s "$changed" ]]; then
	printf 'No changed Go files; duplicate scan skipped.\n'
	exit 0
fi

GOBIN="$tools" go install github.com/mibk/dupl@v1.0.0
output="$(cd "$repo" && "$tools/dupl" -t 100 -plumbing packages apps/player/internal apps/subtitles/internal)"
printf '%s\n' "$output" | python3 "$repo/scripts/quality/filter-duplicates.py" "$changed"
