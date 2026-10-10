#!/usr/bin/env bash
# shellcheck disable=SC2029 # Only validated, fixed-format image names expand into remote commands.
set -euo pipefail

(( $# == 6 )) || { echo 'usage: deploy-nox-local.sh app snapshot revision runtime-hash commit source-repo' >&2; exit 2; }
app="$1"; snapshot="$2"; sha="$3"; runtime_hash="$4"; commit="$5"; source_repo="$6"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/tooling/nox-app.sh
source "$script_dir/nox-app.sh"
load_nox_app "$app"
[[ "$sha" =~ ^[0-9a-f]{40}$ && "$commit" =~ ^[0-9a-f]{40}$ && "$runtime_hash" =~ ^[0-9a-f]{64}$ ]] || exit 2
[[ ${#nox_host} -le 255 && "$nox_host" =~ ^[[:alnum:]][[:alnum:]@._-]*$ ]] || exit 2
for directory in "$snapshot" "$source_repo"; do
  [[ ${#directory} -le 4096 && "$directory" == /* && "$directory" != / && "$directory" != *$'\n'* && -d "$directory" && ! -L "$directory" ]] || exit 2
done
[[ -f "$snapshot/$app_path/Containerfile" && ! -L "$snapshot/$app_path/Containerfile" ]] || exit 2
ssh_options=(-o BatchMode=yes -o ConnectTimeout=8 -o StrictHostKeyChecking=yes)
remote_arch="$(ssh "${ssh_options[@]}" "$nox_host" "docker info --format '{{.Architecture}}'")"
[[ "$remote_arch" == aarch64 || "$remote_arch" == arm64 ]] || { echo 'Nox must use ARM64' >&2; exit 2; }
current="$(ssh "${ssh_options[@]}" "$nox_host" "docker inspect $container --format '{{index .Config.Labels \"org.opencontainers.image.revision\"}}|{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}'" 2>/dev/null || true)"
if [[ "$current" == "$sha|running|healthy" ]]; then
  printf 'level=info operation=nox_local_deploy app=%s snapshot=%s outcome=already_healthy\n' "$app" "$sha"
  exit 0
fi
image="$image_repo:nox-${sha:0:12}"
base="$image_repo:nox-local-runtime"
runtime="$(ssh "${ssh_options[@]}" "$nox_host" "docker image inspect $base --format '{{index .Config.Labels \"io.kinosail.runtime-source\"}}|{{.Architecture}}'" 2>/dev/null || true)"

still_current() {
  local current
  current="$(python3 "$script_dir/nox_local_source.py" "$source_repo" "$app")" || return 75
  [[ "$current" == "$sha" ]] || { echo 'Local build superseded by newer saves' >&2; return 75; }
}

if [[ "$runtime" == "$runtime_hash|arm64" ]]; then
  payload="$snapshot/.nox-payload"
  mkdir "$payload"
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOWORK=off go -C "$snapshot/$app_path" build -mod=readonly -trimpath \
    -ldflags="-s -w -X main.version=local-${sha:0:12}" -o "$payload/kinosail" ./cmd/kinosail
  still_current
  chmod 0555 "$payload/kinosail"
  cat >"$payload/Dockerfile" <<DOCKERFILE
FROM $base
USER root
COPY kinosail /usr/local/bin/kinosail
LABEL org.opencontainers.image.version=local-${sha:0:12}
LABEL org.opencontainers.image.revision=$sha
LABEL io.kinosail.source.commit=$commit
USER kinosail
HEALTHCHECK --interval=2s --timeout=30s --start-period=10s --retries=3 CMD kinosail healthcheck
DOCKERFILE
  tar -C "$payload" -czf "$snapshot/payload.tar.gz" Dockerfile kinosail
  ssh "${ssh_options[@]}" "$nox_host" "docker build --quiet --tag $image -" <"$snapshot/payload.tar.gz"
else
  trap 'podman image rm "$image" >/dev/null 2>&1 || true' EXIT
  podman info >/dev/null 2>&1 || podman machine start >/dev/null
  podman build --layers --platform linux/arm64 --format docker --file "$snapshot/$app_path/Containerfile" \
    --tag "$image" --build-arg "VERSION=local-${sha:0:12}" --build-arg "REVISION=$sha" \
    --label "io.kinosail.runtime-source=$runtime_hash" --label "io.kinosail.source.commit=$commit" "$snapshot"
  still_current
  podman save --format docker-archive "$image" | gzip >"$snapshot/image.tar.gz"
  still_current
  ssh "${ssh_options[@]}" "$nox_host" 'gzip -dc | docker load' <"$snapshot/image.tar.gz"
  ssh "${ssh_options[@]}" "$nox_host" "docker tag $image $base"
fi
still_current
ssh "${ssh_options[@]}" "$nox_host" bash -s -- "$sha" "$image" "$image_repo:nox-dev" "$service" "$container" "$image_repo" <"$script_dir/deploy-nox-remote.sh"
printf 'level=info operation=nox_local_deploy app=%s snapshot=%s outcome=healthy\n' "$app" "$sha"
