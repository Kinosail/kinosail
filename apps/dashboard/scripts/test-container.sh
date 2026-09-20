#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../../../scripts/tooling/gates-pause.sh"
set -euo pipefail

app="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
image="${KINOSAIL_DASHBOARD_TEST_IMAGE:-localhost/kinosail-dashboard:test}"
engine="${CONTAINER_ENGINE:-}"
ready="${KINOSAIL_TEST_IMAGE_READY:-0}"
[[ "$ready" == 0 || "$ready" == 1 ]] || { echo 'invalid image readiness flag' >&2; exit 2; }
container=""
suffix="$$-$RANDOM"
config_volume="kinosail-dashboard-test-$suffix"

if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null; then
    engine="podman"
  elif command -v docker >/dev/null; then
    engine="docker"
  else
    printf 'Podman or Docker is required.\n' >&2
    exit 1
  fi
fi

cleanup() {
  if [[ -n "$container" ]]; then
    "$engine" rm --force "$container" >/dev/null 2>&1 || true
  fi
  "$engine" volume rm --force "$config_volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'printf "Container test failed at line %d.\n" "$LINENO" >&2; [[ -z "$container" ]] || "$engine" logs "$container" >&2' ERR

revision="0123456789abcdef0123456789abcdef01234567"
build=(build --file "$repo/apps/dashboard/Containerfile" --tag "$image" --build-arg VERSION=test --build-arg "REVISION=$revision")
if [[ "$(basename "$engine")" == "podman" ]]; then
  build+=(--format docker)
fi
if [[ "$ready" == 1 ]]; then
  "$engine" image inspect "$image" >/dev/null
else
  "$engine" "${build[@]}" "$repo"
  [[ "$("$engine" image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image")" == "$revision" ]]
fi
[[ "$("$engine" image inspect --format '{{.Config.User}}' "$image")" == "10001:10001" ]]
"$engine" volume create "$config_volume" >/dev/null
container="$("$engine" run --detach --read-only --cap-drop ALL --security-opt no-new-privileges \
  --publish 127.0.0.1::38400 --volume "$config_volume:/config" "$image")"

port="$($engine port "$container" 38400/tcp)"
url="http://127.0.0.1:${port##*:}"
for _ in {1..120}; do
  if curl --fail --silent --max-time 2 "$url/healthz" | grep --quiet '"status":"ok"'; then
    break
  fi
  sleep 0.25
done
curl --fail --silent --max-time 2 "$url/healthz" | grep --quiet '"status":"ok"'
"$engine" exec "$container" /kinosail-dashboard healthcheck
curl --fail --silent --max-time 2 "$url/setup" | grep --quiet 'Kinosail Dashboard'

"$engine" restart "$container" >/dev/null
for _ in {1..120}; do
  curl --fail --silent --max-time 2 "$url/healthz" >/dev/null 2>&1 && break
  sleep 0.25
done
curl --fail --silent --max-time 2 "$url/healthz" >/dev/null
