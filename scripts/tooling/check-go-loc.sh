#!/usr/bin/env bash
set -euo pipefail
exec python3 "$(dirname "${BASH_SOURCE[0]}")/check-file-loc.py" --go-only --strict --scope "${1:-}" --limit "${2:-300}"
