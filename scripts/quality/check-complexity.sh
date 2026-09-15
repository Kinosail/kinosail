#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tools="${TMPDIR:-/tmp}/kinosail-quality-tools"
mkdir -p "$tools"
GOBIN="$tools" go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
GOBIN="$tools" go install github.com/uudashr/gocognit/cmd/gocognit@v1.2.0
paths=()
while IFS= read -r file; do
	[[ "$file" == *_test.go || "$file" == */third_party/* || ! -f "$repo/$file" ]] || paths+=("$file")
done < <(git -C "$repo" ls-files --cached --others --exclude-standard '*.go')

cd "$repo"
status=0
"$tools/gocyclo" -over 21 "${paths[@]}" || status=1
"$tools/gocognit" -over 21 "${paths[@]}" || status=1
exit "$status"
