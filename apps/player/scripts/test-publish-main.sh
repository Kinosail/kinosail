#!/usr/bin/env bash
set -euo pipefail

app="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
publisher="$app/scripts/publish-main.sh"
pre_push="$app/scripts/pre-push-main.sh"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-publisher-test.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

printf 'refs/heads/task a refs/heads/main b\n' | KINOSAIL_VERIFY_PLAN=1 "$pre_push" origin unused >/dev/null

git init --bare "$tmp/remote.git" >/dev/null 2>&1
git init "$tmp/seed" >/dev/null 2>&1
git -C "$tmp/seed" config user.name Test
git -C "$tmp/seed" config user.email test@example.invalid
mkdir -p "$tmp/seed/apps/player/scripts"
cp "$publisher" "$tmp/seed/apps/player/scripts/publish-main.sh"
printf 'base\n' >"$tmp/seed/apps/player/file"
git -C "$tmp/seed" add .
git -C "$tmp/seed" commit -m base >/dev/null 2>&1
git -C "$tmp/seed" branch -M main
git -C "$tmp/seed" remote add origin "$tmp/remote.git"
git -C "$tmp/seed" push -u origin main >/dev/null 2>&1
git --git-dir="$tmp/remote.git" symbolic-ref HEAD refs/heads/main

git clone "$tmp/remote.git" "$tmp/worker" >/dev/null 2>&1
git -C "$tmp/worker" config user.name Test
git -C "$tmp/worker" config user.email test@example.invalid
printf 'safe\n' >>"$tmp/worker/apps/player/file"
git -C "$tmp/worker" commit -am safe >/dev/null 2>&1
(cd "$tmp/worker" && ./apps/player/scripts/publish-main.sh) >/dev/null 2>&1
test "$(git -C "$tmp/worker" rev-parse HEAD)" = "$(git --git-dir="$tmp/remote.git" rev-parse main)"

printf 'dirty\n' >>"$tmp/worker/apps/player/file"
if (cd "$tmp/worker" && ./apps/player/scripts/publish-main.sh) >/dev/null 2>&1; then
  printf 'dirty worktree was published\n' >&2
  exit 1
fi

git clone "$tmp/remote.git" "$tmp/outscope" >/dev/null 2>&1
git -C "$tmp/outscope" config user.name Test
git -C "$tmp/outscope" config user.email test@example.invalid
printf 'root\n' >"$tmp/outscope/root-file"
git -C "$tmp/outscope" add root-file
git -C "$tmp/outscope" commit -m out-of-scope >/dev/null 2>&1
if (cd "$tmp/outscope" && ./apps/player/scripts/publish-main.sh) >/dev/null 2>&1; then
  printf 'out-of-scope commit was published\n' >&2
  exit 1
fi

printf 'publish-main tests passed\n'
