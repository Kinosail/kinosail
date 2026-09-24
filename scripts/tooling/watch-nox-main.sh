#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
app="${1:-}"
if (( $# < 1 || $# > 2 )) || { (( $# == 2 )) && [[ "$2" != --once ]]; }; then
  printf 'usage: %s {player|subtitles|dashboard} [--once]\n' "$0" >&2
  exit 2
fi

# shellcheck source=scripts/tooling/nox-app.sh
source "$script_dir/nox-app.sh"
load_nox_app "$app"
root="$watch_root"
repo_url="$watch_repo_url"
interval="$watch_interval"

[[ "$interval" =~ ^[1-9][0-9]{0,3}$ ]] || { printf 'invalid deployment interval: %s\n' "$interval" >&2; exit 2; }
[[ ${#root} -le 4096 && "$root" == /* && "$root" != / && "$root" != *$'\n'* ]] || { printf 'invalid deployment root\n' >&2; exit 2; }
[[ ${#repo_url} -le 2048 && -n "$repo_url" && "$repo_url" != -* && "$repo_url" != *$'\n'* ]] || { printf 'invalid deployment repository\n' >&2; exit 2; }
mirror="$root/repo.git"
state="$root/deployed"
mkdir -p "$root"

if [[ ! -d "$mirror" ]]; then git clone --mirror "$repo_url" "$mirror" >/dev/null; fi
git --git-dir="$mirror" remote set-url origin "$repo_url"

published_revision() {
  local head="$1" history candidate runs run jobs publication
  history="$(git --git-dir="$mirror" rev-list --first-parent --max-count=100 "$head")" || return 75
  for candidate in $history; do
    [[ "$candidate" =~ ^[0-9a-f]{40}$ ]] || return 75
    runs="$(gh run list --repo Kinosail/kinosail --workflow ci.yml --commit "$candidate" --event push --limit 1 --json databaseId,headSha,status,conclusion)" || return 75
    [[ ${#runs} -le 8192 ]] || return 75
    run="$(jq -er --arg sha "$candidate" 'if length == 1 and .[0].headSha == $sha and .[0].status == "completed" and .[0].conclusion == "success" then .[0].databaseId else empty end' <<<"$runs")" || {
      printf 'Waiting for successful main CI at %s\n' "$candidate" >&2
      return 75
    }
    [[ "$run" =~ ^[1-9][0-9]{0,18}$ ]] || return 75
    jobs="$(gh run view "$run" --repo Kinosail/kinosail --json jobs)" || return 75
    [[ ${#jobs} -le 1048576 ]] || return 75
    publication="$(jq -er --arg name "Publish verified containers / Advance $app production tags" '[.jobs[] | select(.name == $name) | .conclusion] | if length == 0 then "unselected" elif length == 1 and .[0] == "success" then "success" else "invalid" end' <<<"$jobs")" || return 75
    case "$publication" in
      success) printf '%s\n' "$candidate"; return 0 ;;
      unselected) ;;
      *) printf 'No verified %s production image for %s\n' "$app" "$candidate" >&2; return 75 ;;
    esac
  done
  printf 'No verified %s production image in recent main history\n' "$app" >&2
  return 75
}

while true; do
  deploy_status=0
  sha="$(git ls-remote "$repo_url" refs/heads/main | awk '{print $1}')"
  [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { printf 'invalid remote main revision: %s\n' "$sha" >&2; exit 1; }
  if [[ "$sha" != "$(cat "$state" 2>/dev/null || true)" ]]; then
    git --git-dir="$mirror" fetch --quiet origin "+refs/heads/main:refs/heads/main"
    if candidate="$(published_revision "$sha")"; then
      # Recover after a Mac restart or a stopped build VM; concurrent starts are harmless.
      if ! podman info >/dev/null 2>&1; then
        podman machine start >/dev/null 2>&1 || true
      fi
      if env "$git_dir_name=$mirror" KINOSAIL_DEPLOY_EXPECT_MAIN="$sha" "$script_dir/deploy-nox-app.sh" "$app" "$candidate"; then
        printf '%s' "$sha" >"$state"
      else
        deploy_status=$?
      fi
    else
      deploy_status=$?
    fi
  fi
  [[ "${2:-}" == --once ]] && exit "$deploy_status"
  sleep "$interval"
done
