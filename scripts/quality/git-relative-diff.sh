#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == diff && "${2:-}" == --merge-base ]]; then
	exec "$KINOSAIL_REAL_GIT" diff --relative "${@:2}"
fi
exec "$KINOSAIL_REAL_GIT" "$@"
