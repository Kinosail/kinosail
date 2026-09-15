#!/usr/bin/env bash
set -euo pipefail

tmp="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-auto-revert-test.XXXXXX")"
reverter="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/auto-revert-main.sh"
trap 'rm -rf "$tmp"' EXIT
git init --bare "$tmp/remote.git" >/dev/null 2>&1
git init "$tmp/seed" >/dev/null 2>&1
git -C "$tmp/seed" config user.name Test
git -C "$tmp/seed" config user.email test@example.invalid
printf 'base\n' >"$tmp/seed/file"
git -C "$tmp/seed" add file
git -C "$tmp/seed" commit -m base >/dev/null 2>&1
git -C "$tmp/seed" branch -M main
git -C "$tmp/seed" remote add origin "$tmp/remote.git"
git -C "$tmp/seed" push -u origin main >/dev/null 2>&1
printf 'bad\n' >>"$tmp/seed/file"
git -C "$tmp/seed" commit -am bad >/dev/null 2>&1
bad="$(git -C "$tmp/seed" rev-parse HEAD)"
git -C "$tmp/seed" push origin main >/dev/null 2>&1
git clone "$tmp/remote.git" "$tmp/worker" >/dev/null 2>&1
(cd "$tmp/worker" && "$reverter" origin main "$bad") >/dev/null 2>&1
test "$(git --git-dir="$tmp/remote.git" show main:file)" = base
git --git-dir="$tmp/remote.git" show -s --format=%s main | grep -Fq '[auto-revert]'
(cd "$tmp/worker" && "$reverter" origin main "$bad") | grep -Fq 'SKIP auto-revert'
printf 'auto-revert tests passed\n'
