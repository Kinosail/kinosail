#!/usr/bin/env bash
# The file cap is explicitly enabled even while other quality gates are paused.
set -euo pipefail
exec python3 "$(dirname "${BASH_SOURCE[0]}")/../tooling/check-file-loc.py" "$@"
