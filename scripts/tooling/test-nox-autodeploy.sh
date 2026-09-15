#!/usr/bin/env bash
# shellcheck disable=SC2016 # The quoted variables belong to generated mock commands.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-autodeploy-test.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin"
sha=0123456789abcdef0123456789abcdef01234567

printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$tmp/bin/launchctl"
printf '%s\n' '#!/usr/bin/env bash' \
  'if [[ " $* " == *" ls-remote "* ]]; then printf "%s refs/heads/main\n" "$KINOSAIL_TEST_SHA"; fi' \
  'exit 0' >"$tmp/bin/git"
printf '%s\n' '#!/usr/bin/env bash' 'printf "%s|running|healthy\n" "$KINOSAIL_TEST_SHA"' >"$tmp/bin/ssh"
printf '%s\n' '#!/usr/bin/env bash' 'printf "%s\n" "$*" >>"$KINOSAIL_TEST_PODMAN_LOG"' 'exit 99' >"$tmp/bin/podman"
chmod +x "$tmp/bin/"*

reject() {
  local tool="$1"
  shift
  local home="$tmp/rejected-$tool-$#"
  if HOME="$home" PATH="$tmp/bin:$PATH" "$script_dir/$tool" "$@" >/dev/null 2>&1; then
    printf 'invalid auto-deploy input was accepted: %s %s\n' "$tool" "$*" >&2
    exit 1
  fi
  [[ ! -e "$home" ]] || { printf 'invalid auto-deploy input caused a side effect: %s %s\n' "$tool" "$*" >&2; exit 1; }
}

reject install-nox-autodeploy.sh
reject install-nox-autodeploy.sh unknown
reject install-nox-autodeploy.sh player extra
reject watch-nox-main.sh
reject watch-nox-main.sh unknown
reject watch-nox-main.sh player invalid

reject_watch_env() {
  local name="$1" value="$2" home="$tmp/rejected-watch-$1"
  if HOME="$home" PATH="$tmp/bin:$PATH" env "$name=$value" "$script_dir/watch-nox-main.sh" player --once >/dev/null 2>&1; then
    printf 'invalid watcher environment was accepted: %s\n' "$name" >&2
    exit 1
  fi
  [[ ! -e "$home" ]] || { printf 'invalid watcher environment caused a side effect: %s\n' "$name" >&2; exit 1; }
}

reject_watch_env KINOSAIL_DEPLOY_INTERVAL 0
reject_watch_env KINOSAIL_DEPLOY_INTERVAL 10000
reject_watch_env KINOSAIL_DEPLOY_ROOT relative
reject_watch_env KINOSAIL_DEPLOY_ROOT /
reject_watch_env KINOSAIL_DEPLOY_REPO_URL -invalid
reject_watch_env KINOSAIL_DEPLOY_REPO_URL $'https://example.invalid/repo.git\ninvalid'

for app in player subtitles dashboard; do
  home="$tmp/$app"
  HOME="$home" PATH="$tmp/bin:$PATH" "$script_dir/install-nox-autodeploy.sh" "$app" >/dev/null
  install_root="$(find "$home/Library/Application Support" -mindepth 1 -maxdepth 1 -type d)"
  plist="$(find "$home/Library/LaunchAgents" -type f -name '*.plist')"
  [[ -x "$install_root/deploy-nox-app.sh" && -x "$install_root/deploy-nox-remote.sh" ]]
  [[ -x "$install_root/watch-nox-main.sh" && -r "$install_root/nox-app.sh" ]]
  [[ -x "$install_root/scan-deployment-image.sh" ]] || { printf 'installed deployment scanner is missing\n' >&2; exit 1; }
  cmp "$script_dir/scan-deployment-image.sh" "$install_root/scan-deployment-image.sh"
  grep -Fq "<string>$app</string>" "$plist"

  cache="$(find "$home/Library/Caches" -mindepth 1 -maxdepth 1 -type d)"
  mkdir -p "$cache/repo.git"
  HOME="$home" PATH="$tmp/bin:$PATH" KINOSAIL_TEST_SHA="$sha" KINOSAIL_TEST_PODMAN_LOG="$tmp/podman-$app.log" \
    "$install_root/watch-nox-main.sh" "$app" --once >/dev/null
  [[ "$(cat "$cache/deployed")" == "$sha" ]]
  grep -Fxq "machine start" "$tmp/podman-$app.log"
done

printf 'Nox auto-deploy install tests passed\n'
