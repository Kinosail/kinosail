#!/usr/bin/env bash
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
common="$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir)"
hook="$common/hooks/pre-push"
hook_source="$repo/scripts/tooling/pre-push-main.sh"
mode="${1:-status}"
github_repo="$(cd "$repo" && gh repo view --json nameWithOwner --jq .nameWithOwner)"

install_local() {
  install -m 755 "$hook_source" "$hook"
  rm -f "$common/hooks/pre-commit"
}

case "$mode" in
  local)
    command -v podman >/dev/null
    install_local
    gh api --method PUT "repos/$github_repo/actions/permissions" -F enabled=false >/dev/null
    ;;
  status)
    ;;
  *)
    printf 'usage: %s {local|status}\n' "$0" >&2
    exit 2
    ;;
esac

enabled="$(gh api "repos/$github_repo/actions/permissions" --jq='.enabled')"
local=off
[[ -f "$hook" ]] && cmp -s "$hook_source" "$hook" && local=on
printf 'local=%s engine=podman github-actions=%s\n' "$local" "$(if [[ "$enabled" == true ]]; then printf enabled; else printf disabled; fi)"
