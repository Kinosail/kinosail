#!/usr/bin/env bash
set -euo pipefail

tool="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check-go-loc.sh"

for scope in '' unknown apps/../player; do
  if "$tool" "$scope" 300 >/dev/null 2>&1; then
    printf 'invalid Go source scope was accepted: %s\n' "$scope" >&2
    exit 1
  fi
done

for limit in 0 -1 10000 many; do
  if "$tool" packages "$limit" >/dev/null 2>&1; then
    printf 'invalid Go line limit was accepted: %s\n' "$limit" >&2
    exit 1
  fi
done

"$tool" packages 300
printf 'Go line limit tests passed\n'
