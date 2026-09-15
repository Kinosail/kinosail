#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-metrics.XXXXXX")"
trap 'rm -rf "$tools"' EXIT
GOWORK=off go -C "$repo/scripts/quality/metrics" build -o "$tools/metrics" .
python3 "$repo/scripts/quality/check-halstead.py" "$tools/metrics" "$repo"
"$repo/scripts/quality/check-ts-halstead.mjs"
