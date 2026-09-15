#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
app_path="$(git -C "$app" rev-parse --show-prefix)"
app_path="${app_path%/}"
base="${1:-origin/main}"
cd "$app"
git -C "$repo" rev-parse --verify "$base^{commit}" >/dev/null
git -C "$repo" diff --relative="$app_path" --check "$base"...HEAD -- "$app_path"
make check-fast
make test-race
make container-test
make test-instance-check
