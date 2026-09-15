#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/gates-pause.sh"
set -euo pipefail

repo="$(git rev-parse --show-toplevel)"
if git -C "$repo" diff --cached --quiet; then
	printf 'staged quality check skipped: no staged changes\n'
	exit 0
fi

snapshot="$(mktemp -d "${TMPDIR:-/tmp}/kinosail-staged-quality.XXXXXX")"
lease_task="staged-quality-$$"
cleanup() {
	"$repo/scripts/tooling/worktree_guard.py" --repo "$repo" release --task "$lease_task" --worktree "$snapshot" --missing-ok >/dev/null 2>&1 || true
	git -C "$repo" worktree remove --force "$snapshot" >/dev/null 2>&1 || rmdir "$snapshot" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

parent="$(git -C "$repo" rev-parse HEAD)"
tree="$(git -C "$repo" write-tree)"
commit="$({
	printf 'Kinosail staged quality snapshot\n'
} | GIT_AUTHOR_NAME='Kinosail Quality' GIT_AUTHOR_EMAIL='quality@localhost' \
	GIT_COMMITTER_NAME='Kinosail Quality' GIT_COMMITTER_EMAIL='quality@localhost' \
	git -C "$repo" commit-tree "$tree" -p "$parent")"
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX
git -C "$repo" worktree add --detach --quiet "$snapshot" "$commit"
"$repo/scripts/tooling/worktree_guard.py" --repo "$repo" lease --task "$lease_task" --ttl 7200 --worktree "$snapshot"

export KINOSAIL_LINT_BASE="$parent"
export KINOSAIL_MUTATION_DIFF="$parent"
"$snapshot/scripts/quality/check-full.sh"
