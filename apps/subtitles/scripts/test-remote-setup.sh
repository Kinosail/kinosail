#!/usr/bin/env bash
set -euo pipefail

source_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

make_case() {
  local target="$1"
  mkdir -p "$target/scripts"
  cp "$source_root/scripts/setup-remote-access.sh" "$source_root/scripts/disable-remote-access.sh" "$target/scripts/"
  cp "$source_root"/compose{,.config,.release,.remote-https,.remote-wireguard}.yaml "$target/"
  printf '%s\n' 'KINOSAIL_REMOTE_MODE=off' 'KINOSAIL_IMAGE=ghcr.io/mikeo7/kinosail-subtitles@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' >"$target/.env"
}

mkdir -p "$fixture/bin"
export KINOSAIL_REMOTE_SETUP_TEST_LOG="$fixture/commands.log"
cat >"$fixture/bin/podman" <<'FAKE'
#!/usr/bin/env bash
printf 'podman %s\n' "$*" >>"$KINOSAIL_REMOTE_SETUP_TEST_LOG"
if [[ "${KINOSAIL_REMOTE_TEST_FAIL_UP:-}" == yes && " $* " == *" up "* ]]; then exit 1; fi
FAKE
cat >"$fixture/bin/xdg-open" <<'FAKE'
#!/usr/bin/env bash
printf 'open %s\n' "$*" >>"$KINOSAIL_REMOTE_SETUP_TEST_LOG"
FAKE
cat >"$fixture/bin/systemctl" <<'FAKE'
#!/usr/bin/env bash
exit 1
FAKE
cat >"$fixture/bin/sudo" <<'FAKE'
#!/usr/bin/env bash
printf 'sudo %s\n' "$*" >>"$KINOSAIL_REMOTE_SETUP_TEST_LOG"
FAKE
chmod +x "$fixture/bin/podman" "$fixture/bin/xdg-open" "$fixture/bin/systemctl" "$fixture/bin/sudo"

valid="$fixture/valid"
make_case "$valid"
printf 'KINOSAIL_AUTH_URL=https://192.168.1.20:38127\n' >>"$valid/.env"
printf 'version: 1\n' >"$valid/kinosail.yaml"
token=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
printf '\n\n\nfamily-media\n%s\ny\n' "$token" | PATH="$fixture/bin:$PATH" "$valid/scripts/setup-remote-access.sh" >"$fixture/valid.out"
grep -Fq 'Protocol:                TCP only' "$fixture/valid.out"
grep -Fq 'External / public port:  443' "$fixture/valid.out"
grep -Fq 'Internal / private port: 443' "$fixture/valid.out"
grep -Fq 'Set up access away from home' "$fixture/valid.out"
grep -Fq 'Never approve an unexpected code' "$fixture/valid.out"
if grep -Fq "$token" "$fixture/valid.out"; then
  echo "setup output must not expose the DuckDNS token" >&2
  exit 1
fi
grep -Fqx 'KINOSAIL_REMOTE_MODE=https' "$valid/.env"
grep -Fqx 'KINOSAIL_AUTH_URL=https://family-media.duckdns.org' "$valid/.env"
grep -Fqx 'KINOSAIL_REMOTE_LISTEN=:8443' "$valid/.env"
[[ "$(<"$valid/secrets/duckdns_token")" == "$token" ]]
[[ -n "$(find "$valid/secrets/duckdns_token" -perm 600 -print)" ]]
grep -Fq 'podman compose --file compose.release.yaml --file compose.config.yaml --file compose.remote-https.yaml up --detach --force-recreate' "$fixture/commands.log"
grep -Fq 'podman compose --file compose.release.yaml --file compose.config.yaml --file compose.remote-https.yaml exec -T kinosail kinosail healthcheck' "$fixture/commands.log"
# Re-running setup must preserve the original LAN address.
printf '\n\n\n\ny\ny\n' | PATH="$fixture/bin:$PATH" "$valid/scripts/setup-remote-access.sh" >/dev/null
grep -Fqx 'KINOSAIL_LOCAL_AUTH_URL=https://192.168.1.20:38127' "$valid/.env"
# Switching to WireGuard also restores local authentication.
printf '\nwireguard\n\ny\ny\n' | PATH="$fixture/bin:$PATH" "$valid/scripts/setup-remote-access.sh" >/dev/null
grep -Fqx 'KINOSAIL_AUTH_URL=https://192.168.1.20:38127' "$valid/.env"
grep -Fqx 'KINOSAIL_REMOTE_MODE=wireguard' "$valid/.env"
PATH="$fixture/bin:$PATH" "$valid/scripts/disable-remote-access.sh" >/dev/null
grep -Fqx 'KINOSAIL_AUTH_URL=https://192.168.1.20:38127' "$valid/.env"
grep -Fqx 'KINOSAIL_REMOTE_MODE=off' "$valid/.env"
grep -Fq 'stop --timeout 0 kinosail' "$fixture/commands.log"
grep -Fq 'podman compose --file compose.release.yaml --file compose.config.yaml up --detach --force-recreate --remove-orphans' "$fixture/commands.log"

source_case="$fixture/source"
make_case "$source_case"
printf '%s\n' 'KINOSAIL_REMOTE_MODE=off' 'KINOSAIL_IMAGE=localhost/kinosail-subtitles:dev' >"$source_case/.env"
printf '\n\n\nsource-media\n%s\ny\n' "$token" | PATH="$fixture/bin:$PATH" "$source_case/scripts/setup-remote-access.sh" >/dev/null
grep -Fq 'podman compose --file compose.yaml --file compose.remote-https.yaml up --detach --force-recreate' "$fixture/commands.log"
grep -Fq 'podman compose --file compose.yaml --file compose.remote-https.yaml build kinosail' "$fixture/commands.log"
grep -Fq 'up --detach --force-recreate --remove-orphans' "$fixture/commands.log"
# Gateway must use the same freshly built source image, including on a clean setup.
grep -Fqx 'KINOSAIL_IMAGE=localhost/kinosail-subtitles:dev' "$source_case/.env"
grep -Fqx 'KINOSAIL_LOCAL_AUTH_URL=' "$source_case/.env"
PATH="$fixture/bin:$PATH" "$source_case/scripts/disable-remote-access.sh" >/dev/null
grep -Fqx 'KINOSAIL_AUTH_URL=' "$source_case/.env"
grep -Fq 'podman compose --file compose.yaml up --detach --force-recreate --remove-orphans' "$fixture/commands.log"

# If recreation fails, the stop command must already have closed the old listener.
: >"$fixture/commands.log"
if KINOSAIL_REMOTE_TEST_FAIL_UP=yes PATH="$fixture/bin:$PATH" "$valid/scripts/disable-remote-access.sh" >"$fixture/failed-stop.out" 2>&1; then
  echo "failed recreation must not report success" >&2; exit 1
fi
awk '/ stop --timeout 0 kinosail$/ { stopped=1 } / up --detach/ { if (!stopped) exit 1; seen=1 } END { if (!seen) exit 1 }' "$fixture/commands.log"
if grep -Fq 'Secure remote access is off.' "$fixture/failed-stop.out"; then exit 1; fi

for rejected in mode domain token; do
  target="$fixture/$rejected"
  make_case "$target"
  case "$rejected" in
    mode) input=$'\nrelay\n' ;;
    domain) input=$'\n\n\nBAD DOMAIN\n' ;;
    token) input=$'\n\n\nfamily-media\nshort\n' ;;
  esac
  if printf '%s' "$input" | PATH="$fixture/bin:$PATH" "$target/scripts/setup-remote-access.sh" >/dev/null 2>&1; then
    echo "invalid $rejected input must fail" >&2
    exit 1
  fi
  [[ ! -e "$target/secrets" ]]
  [[ "$(wc -l <"$target/.env" | tr -d ' ')" == 2 ]]
done

printf 'remote setup tests passed\n'
