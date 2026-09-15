#!/usr/bin/env bash
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(git -C "$app" rev-parse --show-toplevel)"
app_path="$(git -C "$app" rev-parse --show-prefix)"
app_path="${app_path%/}"
cd "$repo"
if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  printf 'Refusing to publish a dirty worktree; commit only this task first.\n' >&2
  exit 2
fi
git rev-parse --verify HEAD >/dev/null

while true; do
  git fetch origin main
  base="$(git rev-parse origin/main)"
  if [[ "$(git rev-parse HEAD)" == "$base" ]]; then
    printf 'Nothing to publish; HEAD already equals origin/main (%s).\n' "$base"
    exit 0
  fi
  out_of_scope="$(git diff --name-only "$base"...HEAD | grep -Ev "^$app_path/" || true)"
  if [[ -n "$out_of_scope" ]]; then
    printf 'Refusing to publish commits outside %s:\n%s\n' "$app_path" "$out_of_scope" >&2
    exit 2
  fi
  git rebase origin/main
  published="$(git rev-parse HEAD)"
  if git push origin HEAD:main; then
    remote="$(git ls-remote origin refs/heads/main | awk '{print $1}')"
    if [[ "$remote" != "$published" ]]; then
      git fetch origin main
      git merge-base --is-ancestor "$published" origin/main || {
        printf 'Remote verification failed: published=%s remote=%s\n' "$published" "$remote" >&2
        exit 4
      }
    fi
    printf 'Published and verified origin/main at %s\n' "$published"
    exit 0
  fi
  git fetch origin main
  if [[ "$(git rev-parse origin/main)" == "$base" ]]; then
    printf 'Push failed without a concurrent origin/main advance.\n' >&2
    exit 3
  fi
  printf 'Push lost a race; rebasing and retrying.\n'
done
