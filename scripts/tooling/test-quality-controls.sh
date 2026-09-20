#!/usr/bin/env bash
# shellcheck disable=SC2016 # These assertions intentionally match literal shell and Actions expressions.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

require_text() {
  local file="$1"
  local text="$2"
  grep -Fq -- "$text" "$repo/$file" || {
    printf '%s must contain %s\n' "$file" "$text" >&2
    exit 1
  }
}

for script in \
  check-complexity.sh check-coverage.sh check-crap.sh check-deadcode.sh \
  check-duplicates.sh check-full.sh check-halstead.sh check-lint.sh check-loc.sh \
  check-mutation.sh check-script-duplicates.sh check-script-lint.sh check-static.sh \
  check-ts-halstead.mjs check-ts-types.mjs git-relative-diff.sh; do
  [[ -x "$repo/scripts/quality/$script" ]] || {
    printf 'quality gate is not executable: %s\n' "$script" >&2
    exit 1
  }
done

for hook in check-staged-quality.sh pre-commit.sh pre-push-main.sh worktree_guard.py test-worktree-guard.py; do
  [[ -x "$repo/scripts/tooling/$hook" ]] || {
    printf 'hook entrypoint is not executable: %s\n' "$hook" >&2
    exit 1
  }
done

require_text .pre-commit-config.yaml 'entry: ./scripts/tooling/pre-commit.sh'
require_text scripts/tooling/pre-commit.sh 'Full quality suites run in GitHub Actions.'
require_text scripts/tooling/pre-commit.sh 'worktree_guard.py" heartbeat --if-present'
require_text scripts/tooling/pre-commit.sh 'worktree_guard.py" audit'
require_text scripts/tooling/check-staged-quality.sh 'commit-tree "$tree" -p "$parent"'
require_text scripts/tooling/check-staged-quality.sh 'worktree add --detach --quiet "$snapshot" "$commit"'
require_text scripts/tooling/check-staged-quality.sh 'lease --task "$lease_task" --ttl 7200 --worktree "$snapshot"'
require_text scripts/tooling/check-staged-quality.sh 'export KINOSAIL_MUTATION_DIFF="$parent"'
require_text scripts/tooling/check-staged-quality.sh '"$snapshot/scripts/quality/check-full.sh"'
require_text scripts/quality/check-mutation.sh 'PATH="$git_wrapper:$PATH"'
require_text scripts/quality/git-relative-diff.sh 'diff --relative'
require_text scripts/quality/check-mutation.sh '"(LIVED|NOT COVERED|TIMED OUT)"'
require_text scripts/quality/check-coverage.sh '*/archivetest|*/archivetest/*|*/commandtest|*/commandtest/*|*/configurationtest|*/configurationtest/*|*/servertest|*/servertest/*'
require_text scripts/quality/check-crap.sh '*/archivetest|*/archivetest/*|*/commandtest|*/commandtest/*|*/configurationtest|*/configurationtest/*|*/servertest|*/servertest/*'
require_text scripts/quality/check-mutation.sh '(^|/)(archivetest|commandtest|configurationtest|servertest)/'
require_text scripts/quality/check-deadcode.sh '(archivetest|commandtest|configurationtest|servertest)'
require_text Makefile 'install -m 755 scripts/tooling/pre-commit.sh "$$hooks/pre-commit"'
require_text Makefile './scripts/tooling/worktree_guard.py audit'
require_text Makefile './scripts/tooling/test-worktree-guard.py'
for app in player subtitles dashboard; do
  require_text "apps/$app/Makefile" '@$(MAKE) -C ../.. hooks'
done

require_text .github/workflows/quality.yml 'pull_request:'
require_text .github/workflows/quality.yml 'branches: [main]'
require_text .github/workflows/quality.yml 'workflow_call:'
require_text .github/workflows/quality.yml './scripts/quality/check-static.sh'
require_text .github/workflows/quality.yml './scripts/quality/check-coverage.sh'
require_text .github/workflows/quality.yml './scripts/quality/check-crap.sh'
require_text .github/workflows/quality.yml './scripts/quality/check-mutation.sh "${{ matrix.app }}"'

for app in player subtitles dashboard; do
  workflow=".github/workflows/$app-release.yml"
  require_text "$workflow" 'for workflow in quality.yml'
  require_text "$workflow" 'needs: [quality, images]'
done

require_text scripts/quality/check-static.sh 'pnpm --dir "$repo/scripts/quality" install --frozen-lockfile'
require_text scripts/quality/check-script-duplicates.sh '--threshold 0'
require_text scripts/quality/check-duplicates.sh 'filter-duplicates.py'
