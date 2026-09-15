#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f go.mod && ! -f go.work ]]; then
  printf 'skip: no go.mod or go.work yet: %s\n' "$*"
  exit 0
fi

exec "$@"
