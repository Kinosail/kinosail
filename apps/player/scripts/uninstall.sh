#!/usr/bin/env bash
set -euo pipefail
# Compose must use the verified image pin from the installation.
unset KINOSAIL_IMAGE

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if command -v podman >/dev/null && podman compose version >/dev/null 2>&1; then
  compose=(podman compose)
elif command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  compose=(docker compose)
else
  echo "install Podman Compose or Docker Compose first" >&2
  exit 1
fi

cd "$root"
project=("${compose[@]}" --file compose.release.yaml)
remote_mode="$(sed -n 's/^KINOSAIL_REMOTE_MODE=//p' .env 2>/dev/null | tail -1)"
case "${remote_mode:-off}" in
  https) project+=(--file compose.remote-https.yaml) ;;
esac
"${project[@]}" down --remove-orphans
echo "Kinosail is stopped. Configuration volumes, backups, .env, and Library Content were preserved."
