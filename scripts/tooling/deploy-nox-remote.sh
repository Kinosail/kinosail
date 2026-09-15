#!/usr/bin/env bash
set -euo pipefail

if (( $# != 6 )); then
  printf 'usage: %s revision image stable-image service container image-repository\n' "$0" >&2
  exit 2
fi
sha="$1"
image="$2"
stable_image="$3"
service="$4"
container="$5"
image_repo="$6"

[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { printf 'invalid deployment revision: %s\n' "$sha" >&2; exit 2; }
[[ "$image_repo" =~ ^localhost/kinosail(-[a-z]+)?$ ]] || { printf 'invalid image repository: %s\n' "$image_repo" >&2; exit 2; }
[[ "$image" == "$image_repo:nox-${sha:0:12}" ]] || { printf 'invalid deployment image: %s\n' "$image" >&2; exit 2; }
[[ "$stable_image" == "$image_repo:nox-dev" ]] || { printf 'invalid stable image: %s\n' "$stable_image" >&2; exit 2; }
[[ "$service" =~ ^[a-z0-9][a-z0-9-]{0,63}$ ]] || { printf 'invalid Compose service: %s\n' "$service" >&2; exit 2; }
[[ "$container" =~ ^[a-z0-9][a-z0-9-]{0,63}$ ]] || { printf 'invalid container: %s\n' "$container" >&2; exit 2; }
compose_dir="${KINOSAIL_NOX_COMPOSE_DIR:-/home/nox/docker}"
[[ ${#compose_dir} -le 4096 && "$compose_dir" == /* && "$compose_dir" != *$'\n'* ]] || { printf 'invalid Compose directory\n' >&2; exit 2; }

cd -- "$compose_dir"
old="$(docker image inspect "$stable_image" --format '{{.Id}}' 2>/dev/null || true)"
rollback() {
  [[ -z "$old" ]] || docker tag "$old" "$stable_image"
  docker compose up --detach --no-deps --force-recreate "$service" >/dev/null 2>&1 || true
}
trap rollback ERR
docker tag "$image" "$stable_image"
docker compose up --detach --no-deps --force-recreate "$service" >/dev/null 2>&1
for _ in {1..90}; do
  state="$(docker inspect "$container" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}')"
  [[ "$state" == healthy ]] && break
  [[ "$state" != exited && "$state" != dead ]] || false
  sleep 1
done
[[ "$(docker inspect "$container" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}')" == healthy ]]
[[ "$(docker inspect "$container" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')" == "$sha" ]]
trap - ERR
docker images --format '{{.Repository}}:{{.Tag}}' | grep -E "^${image_repo}:nox-[0-9a-f]{12}$" | grep -Fvx "$image" | xargs -r docker image rm >/dev/null 2>&1 || true
