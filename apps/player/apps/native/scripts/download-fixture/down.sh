#!/usr/bin/env bash
set -euo pipefail

native="$(cd "$(dirname "$0")/../.." && pwd)"
player="$(cd "$native/../.." && pwd)"
root="${KINOSAIL_TEST_ROOT:-$player/.kinosail-test}"
if [[ -f "$root/fault-proxy.pid" ]]; then
  kill "$(<"$root/fault-proxy.pid")" 2>/dev/null || true
  rm -f -- "$root/fault-proxy.pid"
fi
cd "$player"
./scripts/test-instance.sh down --volumes
rm -f -- "$root/owner-token"
