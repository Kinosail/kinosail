#!/usr/bin/env bash

wait_container_test_health() {
  local health
  for _ in {1..240}; do
    health="$(curl --fail --silent --insecure --header "Host: $2" "$1/healthz" || true)"
    [[ "$health" == '{"status":"ok"}' ]] && return
    sleep 0.25
  done
  return 1
}

assert_container_test_headers() {
  local headers
  headers="$(curl --fail --silent --insecure --dump-header - --output /dev/null "$1/healthz")"
  if [[ "${KINOSAIL_BROWSER_TEST:-}" != "1" || "$1" == https://* ]]; then
    printf '%s\n' "$headers" | grep -qi '^strict-transport-security: max-age=31536000'
  else
    ! printf '%s\n' "$headers" | grep -qi '^strict-transport-security:'
  fi
  printf '%s\n' "$headers" | grep -qi "^content-security-policy: default-src 'self'"
  printf '%s\n' "$headers" | grep -qi '^x-content-type-options: nosniff'
}

expect_status() {
  local expected="$1"
  shift
  local actual
  local response_file="${media_dir:?container media directory is required}/security-response"
  actual="$(curl --silent --insecure --output "$response_file" --write-out '%{http_code}' "$@")"
  if [[ "$actual" != "$expected" ]]; then
    echo "expected HTTP $expected, got $actual: $(cat "$response_file")" >&2
    return 1
  fi
}
