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
  if [[ "${KINOSAIL_BROWSER_TEST:-}" != "1" ]]; then
    grep -qi '^strict-transport-security: max-age=31536000' <<<"$headers"
  else
    ! grep -qi '^strict-transport-security:' <<<"$headers"
  fi
  grep -qi "^content-security-policy: default-src 'self'" <<<"$headers"
  grep -qi '^x-content-type-options: nosniff' <<<"$headers"
}
