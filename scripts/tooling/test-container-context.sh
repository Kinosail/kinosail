#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
engine="${CONTAINER_ENGINE:-}"
if [[ -z "$engine" ]]; then
  if command -v podman >/dev/null; then engine=podman; else engine=docker; fi
fi
case "$engine" in
  podman|docker) ;;
  *) printf 'unsupported container engine\n' >&2; exit 2 ;;
esac
fixture="$(mktemp -d)"
image="localhost/kinosail-context-test:$$-$RANDOM"
container=""
cleanup() {
  [[ -z "$container" ]] || "$engine" rm "$container" >/dev/null
  "$engine" image rm "$image" >/dev/null 2>&1 || true
  rm -rf -- "$fixture"
}
trap cleanup EXIT

required=(packages/go.mod packages/webassets/assets/icon.svg)
excluded=(.git/config packages/.env packages/webassets/.env.local packages/.verification/coverage.json)
for app in player subtitles dashboard; do
  required+=("apps/$app/go.mod" "apps/$app/cmd/main.go" "apps/$app/internal/server/source.go")
  excluded+=("apps/$app/.env" "apps/$app/apps/native/node_modules/private" "apps/$app/.verification/result")
done
for path in "${required[@]}" "${excluded[@]}"; do
  mkdir -p "$fixture/$(dirname "$path")"
  printf 'synthetic build-context regression fixture\n' >"$fixture/$path"
done
printf 'FROM scratch\nCOPY . /\n' >"$fixture/Containerfile"

for ignore in .dockerignore .containerignore; do
  # Exercise each repository policy with the selected engine's native matcher.
  cp "$repo/$ignore" "$fixture/.dockerignore"
  cp "$repo/$ignore" "$fixture/.containerignore"
  "$engine" build --quiet --tag "$image" "$fixture" >/dev/null
  candidate="kinosail-context-test-$$-$RANDOM"
  "$engine" create --name "$candidate" "$image" /bin/true >/dev/null
  container="$candidate"
  "$engine" export "$container" | tar -tf - >"$fixture/contents"
  for path in "${required[@]}"; do
    grep -Fxq "$path" "$fixture/contents" || { printf 'missing build input: %s\n' "$path" >&2; exit 1; }
  done
  for path in "${excluded[@]}"; do
    if grep -Fxq "$path" "$fixture/contents"; then
      printf 'private or generated file entered build context: %s\n' "$path" >&2
      exit 1
    fi
  done
  "$engine" rm "$container" >/dev/null
  container=""
done
printf 'Container build-context allowlists passed\n'
