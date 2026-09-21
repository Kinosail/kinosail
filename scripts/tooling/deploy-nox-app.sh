#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
app="${1:-}"
if (( $# < 1 || $# > 2 )); then
  printf 'usage: %s {player|subtitles|dashboard} [revision]\n' "$0" >&2
  exit 2
fi

# shellcheck source=scripts/tooling/nox-app.sh
source "$script_dir/nox-app.sh"
load_nox_app "$app"
git_dir="$deploy_git_dir"
host="$nox_host"
success="$deploy_success"

[[ ${#host} -le 255 && "$host" =~ ^[[:alnum:]][[:alnum:]@._-]*$ ]] || { printf 'invalid Nox host: %s\n' "$host" >&2; exit 2; }
sha="${2:-}"
[[ -z "$sha" || "$sha" =~ ^[0-9a-f]{40}$ ]] || { printf 'invalid deployment revision: %s\n' "$sha" >&2; exit 2; }

git_cmd() {
  if [[ -n "$git_dir" ]]; then
    git --git-dir="$git_dir" "$@"
  else
    git -C "$repo" "$@"
  fi
}

if [[ -z "$sha" ]]; then
  sha="$(git_cmd ls-remote origin refs/heads/main | awk '{print $1}')"
fi
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { printf 'invalid deployment revision: %s\n' "$sha" >&2; exit 2; }

image="$image_repo:nox-${sha:0:12}"
stable_image="$image_repo:nox-dev"
tmp=""
log="${TMPDIR:-/tmp}/kinosail-$app-nox-build-${sha:0:12}.log"

cleanup() {
  [[ -n "$tmp" ]] || return 0
  rm -rf -- "$tmp"
  if podman image exists "$image" 2>/dev/null; then
    podman image rm "$image" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

current="$(ssh -o BatchMode=yes -o ConnectTimeout=8 "$host" "docker inspect $container --format '{{index .Config.Labels \"org.opencontainers.image.revision\"}}|{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}'" 2>/dev/null || true)"
if [[ "$current" == "$sha|running|healthy" ]]; then
  printf 'Nox already runs %s\n' "$sha"
  exit
fi

tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-$app-nox.XXXXXX")"
if (( archive_shared )) && git_cmd cat-file -e "$sha:packages/go.mod" 2>/dev/null; then
  git_cmd archive "$sha" -- "$app_path" packages | tar -x -C "$tmp"
  containerfile="$tmp/$app_path/Containerfile"
else
  git_cmd archive "$sha:$app_path" | tar -x -C "$tmp"
  containerfile="$tmp/Containerfile"
fi

started="$(date +%s)"
build_image() {
  podman build "$@" --platform linux/arm64 --format docker --file "$containerfile" --tag "$image" \
    --build-arg "VERSION=nox-${sha:0:12}" --build-arg "REVISION=$sha" "$tmp"
}
if ! build_image >"$log" 2>&1; then
  # A stopped VM can leave an unusable cached intermediate layer. Rebuild it
  # without deleting images or interrupting another app's build.
  if ! grep -Fq "can't stat (or find?) lower layer" "$log" || ! build_image --no-cache >>"$log" 2>&1; then
    printf 'Nox image build failed; log: %s\n' "$log" >&2
    tail -60 "$log" >&2
    exit 1
  fi
fi
rm -f "$log"
printf 'Built Nox image %s (%ss)\n' "$sha" "$(($(date +%s) - started))"

latest="$(git_cmd ls-remote origin refs/heads/main | awk '{print $1}')"
if [[ "$latest" != "$sha" ]]; then
  printf 'Nox build %s superseded by %s; retry latest\n' "$sha" "$latest" >&2
  exit 75
fi

podman save --format docker-archive "$image" >"$tmp/image.tar"
# Persistent local evidence belongs to this immutable source revision. Scanner
# failure or a fixable high/critical finding stops before the remote image load.
if git_cmd cat-file -e "$sha:.gates-disabled" 2>/dev/null; then
  printf "Deployment scan gate is paused (.gates-disabled).\n"
else
  "$script_dir/scan-deployment-image.sh" "$tmp/image.tar" "$repo/.verification/supply-chain/$app-$sha"
fi
ssh -o BatchMode=yes "$host" docker load <"$tmp/image.tar" >/dev/null
ssh -o BatchMode=yes "$host" bash -s -- "$sha" "$image" "$stable_image" "$service" "$container" "$image_repo" <"$script_dir/deploy-nox-remote.sh"

printf '%s %s\n' "$success" "$sha"
