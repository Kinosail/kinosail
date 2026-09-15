#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
native="$repo/apps/player/apps/native"
if [[ -n "${KINOSAIL_MUTATION_DIFF:-}" ]]; then
	changed_production_files="$(git -C "$repo" diff --name-only "$KINOSAIL_MUTATION_DIFF" -- apps/player/apps/native/src | grep -E '\.(ts|tsx)$' | grep -Ev '(^|/)([^/]+\.test\.(ts|tsx)|testing/|[^/]+-test-helpers\.ts)$' || true)"
	if [[ -z "$changed_production_files" ]]; then
		printf 'native mutation check skipped: no changed production TypeScript files\n'
		exit 0
	fi
fi

pnpm --dir "$native" install --frozen-lockfile
mkdir -p "$native/.verification"
pnpm --dir "$native" test:mutation
"$repo/scripts/quality/check-stryker-report.mjs" "$native/.verification/mutation.json"
