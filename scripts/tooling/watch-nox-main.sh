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

while true; do
  deploy_status=0
  sha="$(git ls-remote "$repo_url" refs/heads/main | awk '{print $1}')"
  [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { printf 'invalid remote main revision: %s\n' "$sha" >&2; exit 1; }
  if [[ "$sha" != "$(cat "$state" 2>/dev/null || true)" ]]; then
    git --git-dir="$mirror" fetch --quiet origin "+refs/heads/main:refs/heads/main"
    # Recover after a Mac restart or a stopped build VM; concurrent starts are harmless.
    if ! podman info >/dev/null 2>&1; then
      podman machine start >/dev/null 2>&1 || true
    fi
    if env "$git_dir_name=$mirror" "$script_dir/deploy-nox-app.sh" "$app" "$sha"; then
      printf '%s' "$sha" >"$state"
    else
      deploy_status=$?
    fi
  fi
  [[ "${2:-}" == --once ]] && exit "$deploy_status"
  sleep "$interval"
done
