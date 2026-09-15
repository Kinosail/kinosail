#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
status=0

while IFS= read -r -d '' file; do
	[[ -f "$repo/$file" ]] || continue
	case "$file" in
		*/.codex/*|*/third_party/*|*/testdata/*|*/docs/*|*/engineering/*|*/assets/*) continue ;;
		*.go|*.ts|*.tsx|*.js|*.mjs|*.py|*.sh|*.css|*.html) ;;
		*) continue ;;
	esac
	if [[ "$file" == *.go ]] && head -n 20 "$repo/$file" | grep -Eq '^// Code generated .* DO NOT EDIT\.$'; then
		continue
	fi
	lines="$(wc -l < "$repo/$file" | tr -d ' ')"
	if (( lines >= 300 )); then
		printf '%s: %d lines; maximum is 299\n' "$file" "$lines" >&2
		status=1
	fi
done < <(git -C "$repo" ls-files -co --exclude-standard -z)

exit "$status"
