#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
exec python3 "$repo/scripts/media_server_benchmark.py" "$@"
