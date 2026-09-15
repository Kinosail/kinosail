#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
config="$repo/apps/player/apps/native/eslint.config.js"
files=()
while IFS= read -r file; do
	[[ -f "$repo/$file" ]] || continue
	case "$file" in
	*/hls.min.js|*/htmx.min.js|*.d.ts|*.test.*|*.spec.*) continue ;;
	apps/*/internal/*.js|packages/webassets/static/*.js|apps/player/apps/native/src/*.ts|apps/player/apps/native/src/*.tsx|apps/player/apps/native/plugins/*.js)
		files+=("$repo/$file")
		;;
	esac
done < <(git -C "$repo" ls-files --cached --others --exclude-standard '*.js' '*.ts' '*.tsx')

if (( ${#files[@]} == 0 )); then
	printf 'production script lint found no files\n' >&2
	exit 1
fi
pnpm --dir "$repo/apps/player/apps/native" exec eslint \
	--config "$config" \
	--no-warn-ignored \
	"${files[@]}"
