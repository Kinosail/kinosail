#!/usr/bin/env bash
# WebKit rejects Secure cookies on HTTP localhost. Use real TLS on its disposable
# Linux runner; browser contexts continue to reject untrusted certificates.
browser_fixture_uses_tls() {
  [[ "${KINOSAIL_BROWSER_TEST:-}" == 1 && "${KINOSAIL_BROWSER_PROJECT:-}" == webkit ]]
}

validate_browser_fixture_tls() {
  if ! browser_fixture_uses_tls; then return; fi
  if [[ "${CI:-}" != true || "${GITHUB_ACTIONS:-}" != true || "${RUNNER_OS:-}" != Linux || "$(uname -s)" != Linux ]]; then
    echo 'WebKit container fixtures require the disposable Linux Actions runner for CA trust' >&2
    return 2
  fi
}

trust_browser_fixture_tls() {
  validate_browser_fixture_tls || return
  if ! browser_fixture_uses_tls; then return; fi
  if [[ $# != 4 ]]; then echo 'invalid browser fixture trust arguments' >&2; return 2; fi
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
  install_browser_fixture_ca "$certificate" "$4"
}

trust_native_browser_fixture_tls() {
  validate_browser_fixture_tls || return
  if ! browser_fixture_uses_tls; then return; fi
  if [[ $# != 3 || "${#1}" -gt 4096 || "${#2}" -gt 4096 || "${#3}" -gt 32 || "${1:-}" != /* || ! -f "${1:-}" || -L "${1:-}" || ! -x "${1:-}" || ! -d "${2:-}" || -L "${2:-}" || ! "${3:-}" =~ ^[0-9]+-[0-9]+$ ]]; then
    echo 'invalid native browser fixture trust arguments' >&2
    return 2
  fi
  local certificate="$2/browser-fixture-ca.crt"
  if [[ -L "$certificate" ]]; then echo 'invalid browser fixture export path' >&2; return 2; fi
  "$1" tls-certificate >"$certificate" || return
  install_browser_fixture_ca "$certificate" "$3"
}

install_browser_fixture_ca() {
  local certificate="$1"
  local trust="/usr/local/share/ca-certificates/kinosail-browser-fixture-$2.crt"
  if [[ -e "$trust" ]]; then echo 'browser fixture trust path already exists' >&2; return 2; fi
  if [[ "$(wc -c <"$certificate")" -gt 262144 ]] || grep -q 'PRIVATE KEY' "$certificate" || [[ "$(grep -c '^-----BEGIN CERTIFICATE-----$' "$certificate")" != 1 ]] || ! awk '
    /^-----BEGIN CERTIFICATE-----$/ { if (inside || done) exit 1; inside=1; next }
    /^-----END CERTIFICATE-----$/ { if (!inside) exit 1; inside=0; done=1; next }
    { if (!inside || $0 !~ /^[A-Za-z0-9+\/=]+$/) exit 1 }
    END { if (inside || !done) exit 1 }' "$certificate" || ! openssl x509 -in "$certificate" -noout >/dev/null 2>&1 || ! openssl x509 -in "$certificate" -noout -text | grep -q 'CA:TRUE'; then
    echo 'invalid browser fixture public CA export' >&2
    return 2
  fi
  BROWSER_FIXTURE_CA_PATH="$trust"
  sudo install -m 0644 "$certificate" "$trust" || return
  sudo update-ca-certificates >/dev/null || return
}

remove_browser_fixture_trust() {
  if [[ -z "${BROWSER_FIXTURE_CA_PATH:-}" ]]; then return; fi
  if [[ ! "$BROWSER_FIXTURE_CA_PATH" =~ ^/usr/local/share/ca-certificates/kinosail-browser-fixture-[0-9]+-[0-9]+\.crt$ ]]; then
    echo 'invalid browser fixture cleanup path' >&2
    return 2
  fi
  sudo rm -f -- "$BROWSER_FIXTURE_CA_PATH"
  sudo update-ca-certificates >/dev/null || return
  BROWSER_FIXTURE_CA_PATH=""
}
