#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
native="$repo/apps/player/apps/native"
pnpm --dir "$native" exec jscpd \
	--format javascript,typescript \
	--ignore '**/hls.min.js,**/htmx.min.js' \
	--min-lines 5 \
	--min-tokens 50 \
	--reporters console \
	--threshold 0 \
	"$repo/packages/webassets/static" \
	"$repo/apps/player/internal/server/static" \
	"$repo/apps/subtitles/internal/server/static" \
	"$repo/apps/dashboard/internal/server/web/static" \
	"$native/src" \
	"$native/plugins"
