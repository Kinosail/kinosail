#!/usr/bin/env bash
# shellcheck disable=SC2016 # The generated mock expands the variable when the mock runs.
set -euo pipefail

tool="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/deploy-nox-app.sh"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-deploy-input-test.XXXXXX")"
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin"
sha=0123456789abcdef0123456789abcdef01234567

for command in git ssh podman trivy syft; do
  printf '%s\n' '#!/usr/bin/env bash' ': >"$KINOSAIL_TEST_SIDE_EFFECT"' 'exit 99' >"$tmp/bin/$command"
  chmod +x "$tmp/bin/$command"
done

reject() {
  if PATH="$tmp/bin:$PATH" KINOSAIL_TEST_SIDE_EFFECT="$tmp/side-effect" "$tool" "$@" >/dev/null 2>&1; then
    printf 'invalid deployment input was accepted: %s\n' "$*" >&2
    exit 1
  fi
  [[ ! -e "$tmp/side-effect" ]] || { printf 'invalid deployment input caused a side effect: %s\n' "$*" >&2; exit 1; }
}

reject
reject unknown
reject player invalid
reject player 0123456789abcdef0123456789abcdef01234567 extra
for app in player subtitles dashboard; do
  reject "$app" 0123456789abcdef0123456789abcdef0123456Z
done

for entry in \
  'player KINOSAIL_NOX_HOST' \
  'subtitles KINOSAIL_SUBTITLES_NOX_HOST' \
  'dashboard KINOSAIL_DASHBOARD_NOX_HOST'; do
  read -r app host_variable <<<"$entry"
  if PATH="$tmp/bin:$PATH" KINOSAIL_TEST_SIDE_EFFECT="$tmp/side-effect" \
    env "$host_variable=-oProxyCommand=invalid" "$tool" "$app" 0123456789abcdef0123456789abcdef01234567 >/dev/null 2>&1; then
    printf 'invalid deployment host was accepted: %s\n' "$app" >&2
    exit 1
  fi
  [[ ! -e "$tmp/side-effect" ]] || { printf 'invalid deployment host caused a side effect: %s\n' "$app" >&2; exit 1; }
done

mkdir "$tmp/positive-bin"
tar -cf "$tmp/empty.tar" --files-from /dev/null
printf '%s\n' '#!/usr/bin/env bash' \
  'printf "%s\n" "$*" >>"$KINOSAIL_TEST_GIT_LOG"' \
  'if [[ " $* " == *" ls-remote "* ]]; then printf "%s refs/heads/main\n" "${KINOSAIL_TEST_REMOTE_SHA:-$KINOSAIL_TEST_SHA}"; exit; fi' \
  'if [[ " $* " == *" cat-file "* ]]; then [[ "$KINOSAIL_TEST_SHARED" == 1 ]]; exit; fi' \
  'if [[ " $* " == *" archive "* ]]; then exec /bin/cat "$KINOSAIL_TEST_ARCHIVE"; fi' \
  'exit 0' >"$tmp/positive-bin/git"
printf '%s\n' '#!/usr/bin/env bash' \
  'printf "%s\n" "$*" >>"$KINOSAIL_TEST_SSH_LOG"' \
  'if [[ " $* " == *" docker inspect "* ]]; then printf "old|running|healthy\n"; exit; fi' \
  'exec /bin/cat >/dev/null' >"$tmp/positive-bin/ssh"
printf '%s\n' '#!/usr/bin/env bash' \
  'printf "%s\n" "$*" >>"$KINOSAIL_TEST_PODMAN_LOG"' \
  'if [[ "${1:-} ${2:-}" == "image exists" ]]; then exit 1; fi' \
  'if [[ "${1:-}" == build && "${KINOSAIL_TEST_BUILD_FAIL:-}" == ordinary ]]; then echo "compiler error" >&2; exit 1; fi' \
  'if [[ "${1:-}" == build && "${KINOSAIL_TEST_BUILD_FAIL:-}" == cache && " $* " != *" --no-cache "* ]]; then printf "can\047t stat (or find?) lower layer\n" >&2; exit 1; fi' \
  'if [[ "${1:-}" == save ]]; then printf "image archive"; fi' \
  'exit 0' >"$tmp/positive-bin/podman"
printf '%s\n' '#!/usr/bin/env bash' 'exit "${KINOSAIL_TEST_SCAN_FAIL:-0}"' >"$tmp/positive-bin/trivy"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$tmp/positive-bin/syft"
printf '%s\n' '#!/usr/bin/env bash' \
  'if [[ " $* " == *" run list "* ]]; then printf "[{\"databaseId\":123,\"headSha\":\"%s\",\"status\":\"completed\",\"conclusion\":\"%s\"}]\n" "$KINOSAIL_TEST_SHA" "${KINOSAIL_TEST_CI_CONCLUSION:-success}"; exit; fi' \
  'if [[ " $* " == *" run view "* ]]; then if [[ "${KINOSAIL_TEST_PUBLISHED:-1}" == 1 ]]; then printf "{\"jobs\":[{\"name\":\"Publish verified containers / Advance %s production tags\",\"conclusion\":\"success\"}]}\n" "$KINOSAIL_TEST_APP"; else printf "{\"jobs\":[]}\n"; fi; exit; fi' \
  'exit 99' >"$tmp/positive-bin/gh"
chmod +x "$tmp/positive-bin/"*

positive() {
  local app="$1" git_variable="$2" shared="$3"
  PATH="$tmp/positive-bin:$PATH" KINOSAIL_TEST_ARCHIVE="$tmp/empty.tar" \
    KINOSAIL_TEST_GIT_LOG="$tmp/git.log" KINOSAIL_TEST_PODMAN_LOG="$tmp/podman.log" \
    KINOSAIL_TEST_SHARED="$shared" KINOSAIL_TEST_SHA="$sha" KINOSAIL_TEST_APP="$app" KINOSAIL_TEST_SSH_LOG="$tmp/ssh.log" \
    env "$git_variable=$tmp/repo.git" "$tool" "$app" "$sha" >/dev/null
}

for entry in \
  'player KINOSAIL_DEPLOY_GIT_DIR localhost/kinosail kinosail kinosail' \
  'subtitles KINOSAIL_SUBTITLES_DEPLOY_GIT_DIR localhost/kinosail-subtitles kinosail-subtitles kinosail-subtitles-dev' \
  'dashboard KINOSAIL_DASHBOARD_DEPLOY_GIT_DIR localhost/kinosail-dashboard kinosail-dashboard kinosail-dashboard'; do
  read -r app git_variable image_repo service container <<<"$entry"
  : >"$tmp/git.log"; : >"$tmp/podman.log"; : >"$tmp/ssh.log"
  positive "$app" "$git_variable" 1
  grep -Fq "bash -s -- $sha $image_repo:nox-${sha:0:12} $image_repo:nox-dev $service $container $image_repo" "$tmp/ssh.log"
  grep -Fq "archive $sha -- apps/$app packages" "$tmp/git.log"
done

: >"$tmp/git.log"; : >"$tmp/podman.log"; : >"$tmp/ssh.log"
positive player KINOSAIL_DEPLOY_GIT_DIR 0
grep -Fq "archive $sha:apps/player" "$tmp/git.log"

# A scanner rejection must stop before loading or switching the remote image.
: >"$tmp/ssh.log"
if KINOSAIL_TEST_SCAN_FAIL=1 positive player KINOSAIL_DEPLOY_GIT_DIR 1; then
  echo 'scanner rejection was ignored' >&2; exit 1
fi
if grep -Eq 'docker load|bash -s --' "$tmp/ssh.log"; then
  echo 'scanner rejection reached deployment' >&2; exit 1
fi
# A paused marker cannot bypass the deployment scanner.
: >"$tmp/ssh.log"
if KINOSAIL_TEST_PAUSED=1 KINOSAIL_TEST_SCAN_FAIL=1 positive player KINOSAIL_DEPLOY_GIT_DIR 1; then
  echo 'paused revision bypassed deployment scanner' >&2; exit 1
fi
if grep -Fq 'docker load' "$tmp/ssh.log"; then
  echo 'paused revision reached deployment' >&2; exit 1
fi

for condition in failed-ci unpublished stale; do
  : >"$tmp/ssh.log"; : >"$tmp/podman.log"
  case "$condition" in
    failed-ci) KINOSAIL_TEST_CI_CONCLUSION=failure positive player KINOSAIL_DEPLOY_GIT_DIR 1 && exit 1 || status=$? ;;
    unpublished) KINOSAIL_TEST_PUBLISHED=0 positive player KINOSAIL_DEPLOY_GIT_DIR 1 && exit 1 || status=$? ;;
    stale) KINOSAIL_TEST_REMOTE_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa positive player KINOSAIL_DEPLOY_GIT_DIR 1 && exit 1 || status=$? ;;
  esac
  [[ "$status" == "$(if [[ "$condition" == unpublished ]]; then printf 10; else printf 75; fi)" ]]
  [[ ! -s "$tmp/podman.log" && ! -s "$tmp/ssh.log" ]] || { echo "$condition caused a deployment side effect" >&2; exit 1; }
done
printf 'Nox deployment input and scan-gate tests passed\n'

# Only a missing cached layer triggers a cache-free rebuild.
: >"$tmp/ssh.log"; : >"$tmp/podman.log"
KINOSAIL_TEST_BUILD_FAIL=cache positive player KINOSAIL_DEPLOY_GIT_DIR 1
grep -Fq 'build --no-cache' "$tmp/podman.log"
grep -Fq 'docker load' "$tmp/ssh.log"
: >"$tmp/ssh.log"; : >"$tmp/podman.log"
if KINOSAIL_TEST_BUILD_FAIL=ordinary positive player KINOSAIL_DEPLOY_GIT_DIR 1; then
  echo 'compiler failure was ignored' >&2; exit 1
fi
if grep -Fq -- '--no-cache' "$tmp/podman.log" || grep -Fq 'docker load' "$tmp/ssh.log"; then
  echo 'compiler failure retried or reached deployment' >&2; exit 1
fi
