#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
port="${KINOSAIL_NATIVE_QA_PORT:-4173}"
url="http://127.0.0.1:$port"
server_log="$(mktemp -t kinosail-native-qa.XXXXXX)"
server_pid=""

cleanup() {
  [[ -z "$server_pid" ]] || kill "$server_pid" 2>/dev/null || true
  [[ -z "$server_pid" ]] || wait "$server_pid" 2>/dev/null || true
  rm -f "$server_log"
}
trap cleanup EXIT

cd "$app"
pnpm --dir apps/native install --frozen-lockfile
pnpm --dir e2e install --frozen-lockfile
pnpm --dir apps/native export:web
KINOSAIL_NATIVE_QA_PORT="$port" node apps/native/scripts/qa-server.mjs >"$server_log" 2>&1 &
server_pid="$!"

for _ in {1..60}; do
  if curl --fail --silent --output /dev/null "$url"; then
    break
  fi
  if ! kill -0 "$server_pid" 2>/dev/null; then
    tail -40 "$server_log" >&2
    exit 1
  fi
  sleep 1
done
curl --fail --silent --output /dev/null "$url"

KINOSAIL_NATIVE_URL="$url" \
  KINOSAIL_E2E_URL="$url" \
  KINOSAIL_BROWSER_MATRIX="${KINOSAIL_BROWSER_MATRIX:-full}" \
  KINOSAIL_BROWSER_WORKERS="${KINOSAIL_BROWSER_WORKERS:-1}" \
  pnpm --dir e2e exec playwright test native-client.spec.ts native-client-recovery.spec.ts
