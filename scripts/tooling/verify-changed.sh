#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if (( $# < 1 || $# > 2 )); then
  printf 'usage: %s {apps/player|apps/subtitles} [base-revision]\n' "$0" >&2
  exit 2
fi
app_path="$1"
case "$app_path" in apps/player|apps/subtitles) ;; *) printf 'unknown verification app: %s\n' "$app_path" >&2; exit 2 ;; esac
app="$repo/$app_path"
base="${2:-origin/main}"
common="$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir)"
cache="$common/kinosail-verify-cache/${app_path//\//-}"
logs="$common/kinosail-verify-logs/${app_path//\//-}"
plan="${KINOSAIL_VERIFY_PLAN:-}"
working="${KINOSAIL_VERIFY_WORKTREE:-}"
mkdir -p "$cache" "$logs"
find "$cache" "$logs" -type f -mtime +7 -delete 2>/dev/null || true
cd "$app"
git -C "$repo" rev-parse --verify "$base^{commit}" >/dev/null

changed="$({
  git -C "$repo" diff --relative="$app_path" --name-only --diff-filter=ACMRD "$base"...HEAD -- "$app_path"
  if [[ -n "$working" ]]; then
    git -C "$repo" diff --relative="$app_path" --name-only --diff-filter=ACMRD -- "$app_path"
    git -C "$repo" diff --cached --relative="$app_path" --name-only --diff-filter=ACMRD -- "$app_path"
    git -C "$repo" ls-files --others --exclude-standard -- "$app_path" | sed "s#^$app_path/##"
  fi
} | LC_ALL=C sort -u)"
shared_changed="$({
  git -C "$repo" diff --name-only --diff-filter=ACMRD "$base"...HEAD -- packages go.work go.work.sum
  if [[ -n "$working" ]]; then
    git -C "$repo" diff --name-only --diff-filter=ACMRD -- packages go.work go.work.sum
    git -C "$repo" diff --cached --name-only --diff-filter=ACMRD -- packages go.work go.work.sum
    git -C "$repo" ls-files --others --exclude-standard -- packages
  fi
} | LC_ALL=C sort -u)"

digest() {
  local file
  while IFS= read -r file; do
    printf '%s\0' "$file"
    if [[ -f "$file" ]]; then shasum -a 256 "$file"; else printf 'missing\n'; fi
  done <<<"$1"
  return 0
}

run_stage() {
  local name="$1" inputs="$2" key log started elapsed
  shift 2
  if [[ -n "$plan" ]]; then
    printf 'PLAN %s: %s\n' "$name" "$*"
    return
  fi
  key="$( { printf '%s\0%s\0' "$name" "$*"; digest "$inputs"; } | shasum -a 256 | awk '{print $1}')"
  log="$logs/$key.log"
  if [[ -z "$working" && -f "$cache/$key" ]]; then
    printf 'CACHED %s\n' "$name"
    return
  fi
  started="$(date +%s)"
  if "$@" >"$log" 2>&1; then
    elapsed="$(($(date +%s) - started))"
    : >"$cache/$key"
    printf 'PASS %s (%ss)\n' "$name" "$elapsed"
  else
    elapsed="$(($(date +%s) - started))"
    printf 'FAIL %s (%ss); full log: %s\n' "$name" "$elapsed" "$log" >&2
    tail -60 "$log" >&2
    return 1
  fi
}

run_required() {
  local name="$1" log="$logs/$1-required.log" started elapsed
  shift
  if [[ -n "$plan" ]]; then
    printf 'PLAN %s: %s\n' "$name" "$*"
    return
  fi
  started="$(date +%s)"
  if "$@" >"$log" 2>&1; then
    elapsed="$(($(date +%s) - started))"
    printf 'PASS %s (%ss)\n' "$name" "$elapsed"
  else
    elapsed="$(($(date +%s) - started))"
    printf 'FAIL %s (%ss); full log: %s\n' "$name" "$elapsed" "$log" >&2
    tail -60 "$log" >&2
    return 1
  fi
}

printf 'VERIFY %d changed path(s) against %s\n' "$(grep -c . <<<"$changed" || true)" "$base"
run_required max-loc "$repo/scripts/tooling/check-go-loc.sh" "$app_path" 300
if [[ -n "$plan" ]]; then
  printf 'PLAN diff-check: git diff --check\n'
else
  git -C "$repo" diff --relative="$app_path" --check "$base"...HEAD -- "$app_path"
  [[ -z "$working" ]] || {
    git -C "$repo" diff --relative="$app_path" --check -- "$app_path"
    git -C "$repo" diff --cached --relative="$app_path" --check -- "$app_path"
  }
  printf 'PASS diff-check (0s)\n'
fi

shell_files="$(grep -E '\.sh$' <<<"$changed" || true)"
shell_args=()
while IFS= read -r file; do [[ -z "$file" || ! -f "$file" ]] || shell_args+=("$file"); done <<<"$shell_files"
((${#shell_args[@]} == 0)) || run_stage shellcheck "$shell_files" shellcheck -x -P "$repo" -P SCRIPTDIR "${shell_args[@]}"

remote_setup_files="$(grep -E '^scripts/(setup-remote-access|test-remote-setup)\.sh$' <<<"$changed" || true)"
[[ -z "$remote_setup_files" ]] || run_stage remote-setup "$(git ls-files scripts/setup-remote-access.sh scripts/test-remote-setup.sh 'compose*.yaml')" ./scripts/test-remote-setup.sh

installer_files="$(grep -E '^scripts/(install|uninstall|test-installer)\.sh$|^compose.*\.ya?ml$' <<<"$changed" || true)"
[[ -z "$installer_files" ]] || run_stage installer "$(git ls-files scripts/install.sh scripts/uninstall.sh scripts/test-installer.sh 'compose*.yaml')" ./scripts/test-installer.sh

nox_deploy_files="$(grep -E '^scripts/(deploy-nox|test-deploy-nox)\.sh$' <<<"$changed" || true)"
[[ -z "$nox_deploy_files" ]] || run_stage nox-deploy "$(git ls-files scripts/deploy-nox.sh scripts/test-deploy-nox.sh)" ./scripts/test-deploy-nox.sh

workflow_files="$(grep -E '^\.github/workflows/.*\.ya?ml$' <<<"$changed" || true)"
workflow_args=()
while IFS= read -r file; do [[ -z "$file" ]] || workflow_args+=("$file"); done <<<"$workflow_files"
[[ -z "$workflow_files" ]] || run_stage actionlint "$workflow_files" actionlint "${workflow_args[@]}"

go_files="$(grep -E '\.go$|^go\.(mod|sum)$' <<<"$changed" || true)"
if [[ -n "$go_files" ]]; then
  deleted_go="$(while IFS= read -r file; do [[ -z "$file" || -e "$file" ]] || printf '%s\n' "$file"; done <<<"$(grep -E '\.go$' <<<"$go_files" || true)")"
  if grep -Eq '^go\.(mod|sum)$' <<<"$go_files" || [[ -n "$deleted_go" ]]; then
    run_stage go-all "$(git ls-files '*.go' go.mod go.sum)" ../../scripts/tooling/with-go-module.sh go test ./...
    run_stage vuln "$(git ls-files '*.go' go.mod go.sum)" ../../scripts/tooling/with-go-module.sh govulncheck ./...
  else
    go_dirs="$(grep -E '\.go$' <<<"$go_files" | xargs -n1 dirname | LC_ALL=C sort -u)"
    while IFS= read -r dir; do
      [[ -n "$dir" ]] || continue
      package="./$dir"
      inputs="$(git ls-files "$dir/*.go" go.mod go.sum)"
      tests=""
      while IFS= read -r file; do
        [[ "$file" == "$dir/"*.go ]] || continue
        if [[ "$file" == *_test.go ]]; then
          tests="$tests${tests:+$'\n'}$file"
        else
          stem="${file##*/}"
          stem="${stem%.go}"
          for paired in "$dir/$stem"*_test.go; do [[ -f "$paired" ]] && tests="$tests${tests:+$'\n'}$paired"; done
        fi
      done <<<"$go_files"
      test_args=()
      while IFS= read -r file; do [[ -z "$file" ]] || test_args+=("$file"); done <<<"$tests"
      names=""
      if ((${#test_args[@]})); then
        names="$(sed -nE 's/^func (Test[A-Za-z0-9_]+)\(.*/\1/p' "${test_args[@]}" 2>/dev/null | LC_ALL=C sort -u | paste -sd '|' - || true)"
      fi
      run_stage "go-compile:$dir" "$inputs" ../../scripts/tooling/with-go-module.sh go test -run '^$' "$package"
      if [[ -n "$names" ]]; then
        run_stage "go-focused:$dir" "$inputs" ../../scripts/tooling/with-go-module.sh go test -count=1 -run "^($names)$" "$package"
      else
        run_stage "go-package:$dir" "$inputs" ../../scripts/tooling/with-go-module.sh go test "$package"
      fi
    done <<<"$go_dirs"
  fi
fi

if [[ -n "$shared_changed" ]]; then
  shared_inputs="$({ git -C "$repo" ls-files packages go.work go.work.sum; git -C "$repo" ls-files --others --exclude-standard -- packages; } | LC_ALL=C sort -u | sed 's#^#../../#')"
  [[ "${KINOSAIL_PACKAGES_VERIFIED:-}" == 1 ]] || run_stage shared-packages "$shared_inputs" make -C "$repo/packages" check
  run_stage shared-consumer "$shared_inputs" ../../scripts/tooling/with-go-module.sh go test ./...
fi

css_files="$(grep -E '^internal/server/static/.*\.css$' <<<"$changed" || true)"
while IFS= read -r file; do
  [[ -z "$file" ]] || run_stage "ui-lint:$file" "$file" "$repo/.codex/skills/impeccable/scripts/impeccable" detect --no-advisory "$file"
done <<<"$css_files"

native_files="$(grep -E '^apps/native/' <<<"$changed" | grep -Ev '^apps/native/AGENTS\.md$' || true)"
if [[ "$app_path" == apps/player && -n "$native_files" ]]; then
  native_inputs="$(printf '%s\n%s\n' "$(git ls-files apps/native)" "$native_files" | LC_ALL=C sort -u)"
  run_stage native-client "$native_inputs" make client-check
fi

e2e_files="$(while IFS= read -r file; do
  [[ "$file" == e2e/*.spec.ts && -f "$file" ]] && printf '%s\n' "$file"
done <<<"$changed"; true)"
if [[ -n "$e2e_files" ]]; then
  e2e_args=()
  while IFS= read -r file; do [[ -z "$file" ]] || e2e_args+=("$file"); done <<<"$e2e_files"
  run_stage e2e-list "$(git ls-files 'e2e/*')" pnpm --dir e2e exec playwright test --list "${e2e_args[@]}"
  if [[ -n "${KINOSAIL_E2E_URL:-}" ]]; then
    run_stage e2e-changed "$(git ls-files 'e2e/*')" pnpm --dir e2e exec playwright test "${e2e_args[@]}" --project=chromium
  fi
fi

if [[ -n "$shared_changed" ]] || grep -Eq '^(Containerfile|Containerfile\.test|compose[^/]*\.ya?ml|scripts/test-container\.sh)$' <<<"$changed"; then
  container_inputs="$(printf '%s\n%s\n' "$(git ls-files Containerfile Containerfile.test 'compose*.yaml' 'scripts/test-container.sh' 'cmd/**' 'internal/**')" "${shared_inputs:-}" | LC_ALL=C sort -u)"
  run_stage container-native "$container_inputs" make container-test
fi

printf 'VERIFIED changed paths\n'
