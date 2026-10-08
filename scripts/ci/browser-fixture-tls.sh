#!/usr/bin/env bash
# WebKit rejects Secure cookies on HTTP localhost. Use real TLS on its disposable
# Linux runner; browser contexts continue to reject untrusted certificates.
# Only trust installed by this invocation belongs to its cleanup lifecycle.
BROWSER_FIXTURE_CA_PATH=""
BROWSER_FIXTURE_NODE_CA_PREVIOUS=""
BROWSER_FIXTURE_NODE_CA_WAS_SET=""
BROWSER_FIXTURE_NODE_CA_PATH=""
BROWSER_FIXTURE_NATIVE_PROJECT=""
BROWSER_FIXTURE_NATIVE_WORKSPACE=""
BROWSER_FIXTURE_NATIVE_NONCE=""

browser_fixture_uses_tls() {
  if [[ $# -gt 1 || "${1:-}" != '' && "${1:-}" != fake-provider ]]; then return 2; fi
  [[ "${KINOSAIL_BROWSER_TEST:-}" == 1 && ( "${KINOSAIL_BROWSER_PROJECT:-}" == webkit || "${1:-}" == fake-provider && ( "${KINOSAIL_BROWSER_PROJECT:-}" == chromium || "${KINOSAIL_BROWSER_PROJECT:-}" == firefox ) ) ]]
}

validate_browser_fixture_tls() {
  if [[ $# -gt 1 || "${1:-}" != '' && "${1:-}" != fake-provider ]]; then return 2; fi
  if [[ "${1:-}" == fake-provider && ( "${KINOSAIL_BROWSER_TEST:-}" != 1 || "${KINOSAIL_BROWSER_PROJECT:-}" != chromium && "${KINOSAIL_BROWSER_PROJECT:-}" != firefox && "${KINOSAIL_BROWSER_PROJECT:-}" != webkit ) ]]; then return 2; fi
  if ! browser_fixture_uses_tls "${1:-}"; then return; fi
  if [[ "${CI:-}" != true || "${GITHUB_ACTIONS:-}" != true || "${RUNNER_OS:-}" != Linux || "$(uname -s)" != Linux ]]; then
    echo 'HTTPS container fixtures require the disposable Linux Actions runner for CA trust' >&2
    return 2
  fi
}

trust_browser_fixture_tls() {
  validate_browser_fixture_tls "${5:-}" || return
  if ! browser_fixture_uses_tls "${5:-}"; then return; fi
  if [[ $# != 4 && $# != 5 ]]; then echo 'invalid browser fixture trust arguments' >&2; return 2; fi
  if [[ "${#1}" -gt 4096 || "${#3}" -gt 4096 || "${#4}" -gt 32 || "$(basename "${1:-}")" != docker && "$(basename "${1:-}")" != podman || ! "${2:-}" =~ ^[a-f0-9]{12,64}$ || ! -d "${3:-}" || -L "${3:-}" || ! "${4:-}" =~ ^[0-9]+-[0-9]+$ ]]; then
    echo 'invalid browser fixture trust arguments' >&2
    return 2
  fi
  local certificate="$3/browser-fixture-ca.crt"
  local trust="/usr/local/share/ca-certificates/kinosail-browser-fixture-$4.crt"
  if [[ -e "$trust" ]]; then echo 'browser fixture trust path already exists' >&2; return 2; fi
  # The app command exports its public CA only, never its private identity.
  if [[ -L "$certificate" ]]; then echo 'invalid browser fixture export path' >&2; return 2; fi
  "$1" exec "$2" kinosail tls-certificate >"$certificate" || return
  install_browser_fixture_ca "$certificate" "$4" || return
  if [[ "${5:-}" == fake-provider && "$KINOSAIL_BROWSER_PROJECT" != webkit ]]; then
    BROWSER_FIXTURE_NATIVE_PROJECT="$KINOSAIL_BROWSER_PROJECT"
    BROWSER_FIXTURE_NATIVE_WORKSPACE="$3"
    BROWSER_FIXTURE_NATIVE_NONCE="$4"
    install_browser_native_ca "$BROWSER_FIXTURE_NATIVE_PROJECT" "$certificate" "$3" "$4"
  fi
}

validate_browser_fixture_ca() {
  if [[ $# != 1 || "${#1}" -gt 4096 || "${1:-}" != /* || ! -f "${1:-}" || -L "${1:-}" ]]; then
    echo 'invalid browser fixture public CA path' >&2
    return 2
  fi
  local certificate="$1"
  if [[ "$(wc -c <"$certificate")" -gt 262144 ]] || grep -q 'PRIVATE KEY' "$certificate" || [[ "$(grep -c '^-----BEGIN CERTIFICATE-----$' "$certificate")" != 1 ]] || ! awk '
    /^-----BEGIN CERTIFICATE-----$/ { if (inside || done) exit 1; inside=1; next }
    /^-----END CERTIFICATE-----$/ { if (!inside) exit 1; inside=0; done=1; next }
    { if (!inside || $0 !~ /^[A-Za-z0-9+\/=]+$/) exit 1 }
    END { if (inside || !done) exit 1 }' "$certificate" || ! openssl x509 -in "$certificate" -noout >/dev/null 2>&1 || ! openssl x509 -in "$certificate" -noout -ext basicConstraints | grep -Eq '^[[:space:]]*CA:TRUE(, pathlen:[0-9]+)?[[:space:]]*$'; then
    echo 'invalid browser fixture public CA export' >&2
    return 2
  fi
}

install_browser_fixture_ca() {
  if [[ $# != 2 || "${#2}" -gt 32 || ! "${2:-}" =~ ^[0-9]+-[0-9]+$ ]]; then
    echo 'invalid browser fixture trust arguments' >&2
    return 2
  fi
  local certificate="$1"
  local trust="/usr/local/share/ca-certificates/kinosail-browser-fixture-$2.crt"
  if [[ -e "$trust" ]]; then echo 'browser fixture trust path already exists' >&2; return 2; fi
  validate_browser_fixture_ca "$certificate" || return
  BROWSER_FIXTURE_CA_PATH="$trust"
  sudo install -m 0644 "$certificate" "$trust" || return
  sudo update-ca-certificates >/dev/null || return
  # Playwright's APIRequestContext runs in Node, outside the browser trust store.
  BROWSER_FIXTURE_NODE_CA_PREVIOUS="${NODE_EXTRA_CA_CERTS-}"
  BROWSER_FIXTURE_NODE_CA_WAS_SET="${NODE_EXTRA_CA_CERTS+x}"
  BROWSER_FIXTURE_NODE_CA_PATH="$certificate"
  export NODE_EXTRA_CA_CERTS="$certificate"
}

install_browser_native_ca() {
  python3 "${BASH_SOURCE[0]%/*}/browser-native-ca.py" install "$@"
}

remove_browser_native_ca() {
  python3 "${BASH_SOURCE[0]%/*}/browser-native-ca.py" remove "$BROWSER_FIXTURE_NATIVE_PROJECT" \
    "$BROWSER_FIXTURE_NATIVE_WORKSPACE/browser-fixture-ca.crt" "$BROWSER_FIXTURE_NATIVE_WORKSPACE" "$BROWSER_FIXTURE_NATIVE_NONCE"
}

remove_browser_fixture_trust() {
  local failed=0
  if [[ -n "$BROWSER_FIXTURE_NATIVE_PROJECT" ]]; then
    if remove_browser_native_ca; then
      BROWSER_FIXTURE_NATIVE_PROJECT=""
      BROWSER_FIXTURE_NATIVE_WORKSPACE=""
      BROWSER_FIXTURE_NATIVE_NONCE=""
    else failed=1; fi
  fi
  if [[ -z "${BROWSER_FIXTURE_CA_PATH:-}" ]]; then return "$failed"; fi
  if [[ ! "$BROWSER_FIXTURE_CA_PATH" =~ ^/usr/local/share/ca-certificates/kinosail-browser-fixture-[0-9]+-[0-9]+\.crt$ ]]; then
    echo 'invalid browser fixture cleanup path' >&2
    return 2
  fi
  sudo rm -f -- "$BROWSER_FIXTURE_CA_PATH" || return
  sudo update-ca-certificates >/dev/null || return
  BROWSER_FIXTURE_CA_PATH=""
  if [[ "${NODE_EXTRA_CA_CERTS-}" == "${BROWSER_FIXTURE_NODE_CA_PATH-}" && -n "${BROWSER_FIXTURE_NODE_CA_PATH-}" ]]; then
    if [[ "${BROWSER_FIXTURE_NODE_CA_WAS_SET-}" == x ]]; then
      export NODE_EXTRA_CA_CERTS="$BROWSER_FIXTURE_NODE_CA_PREVIOUS"
    else
      unset NODE_EXTRA_CA_CERTS
    fi
  fi
  BROWSER_FIXTURE_NODE_CA_PATH=""
  return "$failed"
}

trust_native_browser_fixture_tls() {
  validate_browser_fixture_tls || return
  if ! browser_fixture_uses_tls; then return; fi
  if [[ $# != 3 || "${#1}" -gt 4096 || "${#2}" -gt 4096 || "${#3}" -gt 32 || "${1:-}" != /* || ! -f "${1:-}" || ! -x "${1:-}" || -L "${1:-}" || "${2:-}" != /* || ! -d "${2:-}" || -L "${2:-}" || ! "${3:-}" =~ ^[0-9]+-[0-9]+$ ]]; then
    echo 'invalid native browser fixture trust arguments' >&2
    return 2
  fi
  local certificate="$2/browser-fixture-ca.crt"
  if [[ -L "$certificate" ]]; then echo 'invalid browser fixture export path' >&2; return 2; fi
  "$1" tls-certificate >"$certificate" || return
  install_browser_fixture_ca "$certificate" "$3"
}

# The installing shell owns cleanup; EOF from its Python caller ends that lease.
hold_native_browser_fixture_tls() {
  trap remove_browser_fixture_trust EXIT
  trap 'exit 143' TERM
  trap 'exit 130' INT
  trust_native_browser_fixture_tls "$@" || return
  printf 'ready\n'
  while IFS= read -r; do :; done
}
