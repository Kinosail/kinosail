#!/bin/sh
set -eu

test_data_dir="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-dashboard-e2e.XXXXXX")"
test_server_pid=""
cleanup() {
  trap - EXIT HUP INT TERM
  if [ -n "$test_server_pid" ]; then
    kill "$test_server_pid" 2>/dev/null || true
    wait "$test_server_pid" 2>/dev/null || true
  fi
  rm -rf "$test_data_dir"
}
trap cleanup EXIT HUP INT TERM

export KINOSAIL_DASHBOARD_DATA_DIR="$test_data_dir"
export KINOSAIL_DASHBOARD_LISTEN="${KINOSAIL_DASHBOARD_E2E_LISTEN:-127.0.0.1:38491}"
export KINOSAIL_DASHBOARD_PROBE_INTERVAL="1h"

cd ..
go build -o "$test_data_dir/kinosail-dashboard" ./cmd/kinosail-dashboard
"$test_data_dir/kinosail-dashboard" &
test_server_pid=$!
wait "$test_server_pid"
