#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
base="${KINOSAIL_LINT_BASE:-}"
if [[ -z "$base" ]]; then
	if upstream="$(git -C "$repo" rev-parse --verify origin/main 2>/dev/null)" && [[ "$upstream" != "$(git -C "$repo" rev-parse HEAD)" ]]; then
		base="$(git -C "$repo" merge-base HEAD "$upstream")"
	else
		base=HEAD^
	fi
fi
if [[ "$base" =~ ^0+$ ]] || ! base="$(git -C "$repo" rev-parse --verify --end-of-options "$base^{commit}" 2>/dev/null)"; then
	base="$(git -C "$repo" rev-list --max-parents=0 HEAD | tail -1)"
fi

for app in player subtitles; do
	(cd "$repo/apps/$app" && golangci-lint run --config "$repo/.golangci.yml" --new-from-rev "$base")
done

(cd "$repo/packages" && golangci-lint run --config "$repo/.golangci.yml" --new-from-rev "$base")
