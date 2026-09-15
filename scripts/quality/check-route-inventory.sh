#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo"

go run ./packages/cmd/routeinventory ./apps/player/internal/server ./packages >/dev/null
go run ./packages/cmd/routeinventory ./apps/subtitles/internal/server ./packages >/dev/null
