#!/usr/bin/env bash
# shellcheck disable=SC2016 # The quoted variables belong to generated test commands.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$script_dir/../.." && pwd)"
if (( $# != 1 )); then
  printf 'usage: %s {player|subtitles|dashboard}\n' "$0" >&2
  exit 2
fi
app="$1"
# shellcheck source=scripts/tooling/nox-app.sh
source "$script_dir/nox-app.sh"
load_nox_app "$app"
app_root="$repo/$app_path"

tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-$app-adapter-test.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin"
sha=0123456789abcdef0123456789abcdef01234567
tar -cf "$tmp/empty.tar" --files-from /dev/null

printf '%s\n' '#!/usr/bin/env bash' 'printf "%s\n" "$KINOSAIL_TEST_NOX_STATE"' >"$tmp/bin/ssh"
printf '%s\n' '#!/usr/bin/env bash' \
  'if [[ " $* " == *" ls-remote "* ]]; then printf "%s refs/heads/main\n" "$KINOSAIL_TEST_SHA"; exit; fi' \
  'if [[ " $* " == *" fetch "* || " $* " == *" remote set-url "* ]]; then exit; fi' \
  'if [[ " $* " == *" archive "* ]]; then exec /bin/cat "$KINOSAIL_TEST_ARCHIVE"; fi' \
  'exit 64' >"$tmp/bin/git"
printf '%s\n' '#!/usr/bin/env bash' \
  'printf "%s\n" "$*" >>"$KINOSAIL_TEST_PODMAN_LOG"' \
  '[[ "${1:-} ${2:-}" == "image exists" ]] && exit 1' \
  'exit 42' >"$tmp/bin/podman"
chmod +x "$tmp/bin/"*

deploy() {
  PATH="$tmp/bin:$PATH" KINOSAIL_TEST_ARCHIVE="$tmp/empty.tar" \
    KINOSAIL_TEST_NOX_STATE="$1" KINOSAIL_TEST_PODMAN_LOG="$tmp/podman.log" \
    KINOSAIL_TEST_SHA="$sha" env "$git_dir_name=$tmp/repo.git" "$app_root/scripts/deploy-nox.sh" "$sha"
}

output="$(deploy "$sha|running|healthy")"
[[ "$output" == *"Nox already runs $sha" ]]
[[ ! -e "$tmp/podman.log" ]]

for current in "$sha|running|unhealthy" "$sha|exited|"; do
  : >"$tmp/podman.log"
  if deploy "$current" >/dev/null 2>&1; then
    printf 'invalid Nox state was accepted: %s\n' "$current" >&2
    exit 1
  fi
  grep -q '^build ' "$tmp/podman.log"
done

watch_test_root="$tmp/watch"
mkdir -p "$watch_test_root/repo.git"
set +e
PATH="$tmp/bin:$PATH" KINOSAIL_TEST_ARCHIVE="$tmp/empty.tar" KINOSAIL_TEST_NOX_STATE="" \
  KINOSAIL_TEST_PODMAN_LOG="$tmp/podman.log" KINOSAIL_TEST_SHA="$sha" \
  env "$root_variable=$watch_test_root" "$repo_variable=https://kinosail.test/$app.git" \
  "$app_root/scripts/watch-nox-main.sh" --once >/dev/null 2>&1
watch_status=$?
set -e
[[ "$watch_status" == 1 ]]
[[ ! -e "$watch_test_root/deployed" ]]

grep -Fq "deploy-nox-app.sh\" $app" "$app_root/scripts/deploy-nox.sh"
grep -Fq "watch-nox-main.sh\" $app" "$app_root/scripts/watch-nox-main.sh"
grep -Fq "install-nox-autodeploy.sh\" $app" "$app_root/scripts/install-nox-autodeploy.sh"
printf '%s Nox adapter tests passed\n' "$app"
