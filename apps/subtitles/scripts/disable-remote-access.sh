#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
[[ -f .env ]] || { echo "Kinosail is not installed in $root" >&2; exit 1; }
if command -v podman >/dev/null && podman compose version >/dev/null 2>&1; then
  compose=(podman compose)
elif command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  compose=(docker compose)
else
  echo "install Podman Compose or Docker Compose first" >&2
  exit 1
fi
temporary="$(mktemp "$root/.env.XXXXXX")"
image="$(sed -n 's/^KINOSAIL_IMAGE=//p' .env | tail -1)"
compose_file=compose.yaml
[[ "$image" =~ ^ghcr\.io/kinosail/kinosail-subtitles@sha256:[a-f0-9]{64}$ ]] && compose_file=compose.release.yaml
local_auth="$(sed -n 's/^KINOSAIL_LOCAL_AUTH_URL=//p' .env | tail -1)"
if ! grep -q '^KINOSAIL_LOCAL_AUTH_URL=' .env && ! grep -q '^KINOSAIL_REMOTE_MODE=https$' .env; then
  local_auth="$(sed -n 's/^KINOSAIL_AUTH_URL=//p' .env | tail -1)"
fi
KINOSAIL_RESTORE_AUTH="$local_auth" awk '
  BEGIN { mode = auth = 0; local_auth = ENVIRON["KINOSAIL_RESTORE_AUTH"] }
  /^KINOSAIL_REMOTE_MODE=/ { print "KINOSAIL_REMOTE_MODE=off"; mode = 1; next }
  /^KINOSAIL_AUTH_URL=/ { print "KINOSAIL_AUTH_URL=" local_auth; auth = 1; next }
  { print }
  END {
    if (!mode) print "KINOSAIL_REMOTE_MODE=off"
    if (!auth) print "KINOSAIL_AUTH_URL=" local_auth
  }
' .env >"$temporary"
chmod 600 "$temporary"
mv "$temporary" .env
project=("${compose[@]}" --file "$compose_file")
[[ ! -f kinosail.yaml ]] || project+=(--file compose.config.yaml)
echo "Stopping remote access. Remove the Kinosail port-forwarding rule from your router too."
# Stop first so a failed recreation cannot leave the previous public listener running.
"${project[@]}" stop --timeout 0 kinosail
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet wg-quick@kinosail 2>/dev/null; then
	  sudo systemctl stop wg-quick@kinosail
fi
"${project[@]}" up --detach --force-recreate --remove-orphans
echo "Secure remote access is off. Local Kinosail access is restored."
echo "Remove the Kinosail port-forwarding rule from your router too."
