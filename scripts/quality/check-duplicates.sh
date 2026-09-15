#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="${TMPDIR:-/tmp}/kinosail-quality-tools"
mkdir -p "$tools"
GOBIN="$tools" go install github.com/mibk/dupl@v1.0.0
# The Player and Subtitles server modules intentionally share mirrored product
# adapters. Their per-module golangci runs still enforce duplication within
# each app; this repository-level check focuses on the shared packages where a
# duplicate would otherwise be hidden across package boundaries.
output="$(cd "$repo" && "$tools/dupl" -t 100 -plumbing packages)"
if [[ -n "$output" ]]; then
	printf '%s\n' "$output" >&2
	exit 1
fi
