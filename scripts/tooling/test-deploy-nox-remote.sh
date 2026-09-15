#!/usr/bin/env bash
# shellcheck disable=SC2016 # The quoted variables belong to the generated Docker mock.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool="$script_dir/deploy-nox-remote.sh"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-remote-deploy-test.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin" "$tmp/compose"
sha=0123456789abcdef0123456789abcdef01234567
image="localhost/kinosail:nox-${sha:0:12}"
stable=localhost/kinosail:nox-dev

printf '%s\n' '#!/usr/bin/env bash' \
  'printf "%s\n" "$*" >>"$KINOSAIL_TEST_DOCKER_LOG"' \
  'if [[ " $* " == *" image inspect "* ]]; then printf "old-image\n"; exit; fi' \
  'if [[ " $* " == *" compose up "* ]]; then' \
  '  count=$(cat "$KINOSAIL_TEST_COMPOSE_COUNT" 2>/dev/null || printf 0)' \
  '  count=$((count + 1)); printf "%s" "$count" >"$KINOSAIL_TEST_COMPOSE_COUNT"' \
  '  [[ "${KINOSAIL_TEST_FAIL_FIRST_COMPOSE:-}" == 1 && "$count" == 1 ]] && exit 1' \
  '  exit 0' \
  'fi' \
  'if [[ " $* " == *" inspect "* && " $* " == *"Health.Status"* ]]; then printf "%s\n" "${KINOSAIL_TEST_STATE:-healthy}"; exit; fi' \
  'if [[ " $* " == *" inspect "* && " $* " == *"image.revision"* ]]; then printf "%s\n" "$KINOSAIL_TEST_SHA"; exit; fi' \
  'if [[ " $* " == *" images "* ]]; then exit; fi' \
  'exit 0' >"$tmp/bin/docker"
chmod +x "$tmp/bin/docker"

run_remote() {
  PATH="$tmp/bin:$PATH" KINOSAIL_NOX_COMPOSE_DIR="${KINOSAIL_NOX_COMPOSE_DIR:-$tmp/compose}" \
    KINOSAIL_TEST_DOCKER_LOG="$tmp/docker.log" KINOSAIL_TEST_COMPOSE_COUNT="$tmp/compose-count" \
    KINOSAIL_TEST_SHA="$sha" KINOSAIL_TEST_STATE="${KINOSAIL_TEST_STATE:-healthy}" "$tool" "$@"
}

reject() {
  rm -f "$tmp/docker.log"
  if run_remote "$@" >/dev/null 2>&1; then
    printf 'invalid remote deployment input was accepted: %s\n' "$*" >&2
    exit 1
  fi
  [[ ! -e "$tmp/docker.log" ]] || { printf 'invalid remote deployment input caused a side effect: %s\n' "$*" >&2; exit 1; }
}

reject invalid "$image" "$stable" kinosail kinosail localhost/kinosail
reject "$sha" invalid "$stable" kinosail kinosail localhost/kinosail
reject "$sha" "$image" invalid kinosail kinosail localhost/kinosail
reject "$sha" "$image" "$stable" Invalid kinosail localhost/kinosail
reject "$sha" "$image" "$stable" kinosail Invalid localhost/kinosail
reject "$sha" "$image" "$stable" kinosail kinosail invalid/repository
if KINOSAIL_NOX_COMPOSE_DIR=relative run_remote "$sha" "$image" "$stable" kinosail kinosail localhost/kinosail >/dev/null 2>&1; then
  printf 'relative Compose directory was accepted\n' >&2
  exit 1
fi
[[ ! -e "$tmp/docker.log" ]]

run_remote "$sha" "$image" "$stable" kinosail kinosail localhost/kinosail
grep -Fq "tag $image $stable" "$tmp/docker.log"
grep -Fq 'compose up --detach --no-deps --force-recreate kinosail' "$tmp/docker.log"

: >"$tmp/docker.log"
rm -f "$tmp/compose-count"
if PATH="$tmp/bin:$PATH" KINOSAIL_NOX_COMPOSE_DIR="$tmp/compose" \
  KINOSAIL_TEST_DOCKER_LOG="$tmp/docker.log" KINOSAIL_TEST_COMPOSE_COUNT="$tmp/compose-count" \
  KINOSAIL_TEST_FAIL_FIRST_COMPOSE=1 KINOSAIL_TEST_SHA="$sha" \
  "$tool" "$sha" "$image" "$stable" kinosail kinosail localhost/kinosail >/dev/null 2>&1; then
  printf 'failed remote deployment did not fail\n' >&2
  exit 1
fi
grep -Fq "tag old-image $stable" "$tmp/docker.log"
[[ "$(grep -Fc 'compose up --detach --no-deps --force-recreate kinosail' "$tmp/docker.log")" == 2 ]]

for state in exited dead; do
  : >"$tmp/docker.log"
  rm -f "$tmp/compose-count"
  if KINOSAIL_TEST_STATE="$state" run_remote "$sha" "$image" "$stable" kinosail kinosail localhost/kinosail >/dev/null 2>&1; then
    printf 'terminal container state was accepted: %s\n' "$state" >&2
    exit 1
  fi
  grep -Fq "tag old-image $stable" "$tmp/docker.log"
  [[ "$(grep -Fc 'compose up --detach --no-deps --force-recreate kinosail' "$tmp/docker.log")" == 2 ]]
done

printf 'Nox remote deployment tests passed\n'
