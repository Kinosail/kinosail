#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

quality="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$quality/../.." && pwd)"
python3 "$quality/check-dependency-integrity.py"
"$quality/check-lint.sh"
"$quality/check-loc.sh"
pnpm --dir "$repo/apps/player/apps/native" install --frozen-lockfile
"$quality/check-ts-types.mjs"
"$quality/check-script-lint.sh"
"$quality/check-complexity.sh"
"$quality/check-halstead.sh"
"$quality/check-route-inventory.sh"
"$quality/check-deadcode.sh"
"$quality/check-duplicates.sh"
"$quality/check-script-duplicates.sh"
