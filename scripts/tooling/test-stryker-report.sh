#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
report="$(mktemp "${TMPDIR:-/tmp}/kinosail-stryker-report.XXXXXX")"
trap 'rm -f "$report"' EXIT INT TERM

write_report() {
	printf '{"files":{"source.ts":{"source":"source","mutants":[%s]}}}\n' "$1" >"$report"
}

write_report '{"status":"Killed","location":{"start":{"line":1}}}'
"$repo/scripts/quality/check-stryker-report.mjs" "$report"

write_report '{"status":"Ignored","statusReason":"equivalent","location":{"start":{"line":1}}},{"status":"Killed","location":{"start":{"line":1}}}'
"$repo/scripts/quality/check-stryker-report.mjs" "$report"

write_report '{"status":"Timeout","location":{"start":{"line":1}}}'
"$repo/scripts/quality/check-stryker-report.mjs" "$report"

write_report '{"status":"Survived","location":{"start":{"line":1}}}'
if "$repo/scripts/quality/check-stryker-report.mjs" "$report" >/dev/null 2>&1; then
	printf 'surviving mutant was accepted\n' >&2
	exit 1
fi

write_report '{"status":"Ignored","location":{"start":{"line":1}}}'
if "$repo/scripts/quality/check-stryker-report.mjs" "$report" >/dev/null 2>&1; then
	printf 'unjustified ignored mutant was accepted\n' >&2
	exit 1
fi

printf '{"files":{}}\n' >"$report"
if "$repo/scripts/quality/check-stryker-report.mjs" "$report" >/dev/null 2>&1; then
	printf 'empty mutation report was accepted\n' >&2
	exit 1
fi

printf 'Stryker report tests passed\n'
