#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
# shellcheck disable=SC2016 # Assertions intentionally match literal Actions and shell expressions.
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
workflows="$repo/.github/workflows"
fail() { printf 'workflow validation failed: %s\n' "$*" >&2; exit 1; }
require() { grep -Fq -- "$2" "$1" || fail "$(basename "$1") must contain $2"; }

for name in ci deep publish release layout-stability; do
  [[ -f "$workflows/$name.yml" ]] || fail "missing $name.yml"
done
[[ "$(find "$workflows" -maxdepth 1 -name '*.yml' -type f | wc -l | tr -d ' ')" == 5 ]] ||
  fail 'unexpected workflow file'
[[ -f "$repo/.github/dependabot.yml" ]] || fail 'missing Dependabot configuration'
[[ -f "$repo/.github/pull_request_template.md" ]] || fail 'missing pull request template'

ci="$workflows/ci.yml"
deep="$workflows/deep.yml"
publish="$workflows/publish.yml"
release="$workflows/release.yml"
require "$ci" '  pull_request:'
require "$ci" '    branches: [main]'
require "$ci" '  merge_group:'
require "$ci" 'run: python3 scripts/ci/plan.py'
require "$ci" 'run: python3 scripts/ci/required.py repository'
require "$ci" 'run: python3 scripts/ci/required.py app'
require "$ci" 'run: python3 scripts/ci/required.py security'
require "$ci" 'results: ${{ toJSON(needs) }}'
require "$ci" 'uses: ./.github/workflows/publish.yml'
require "$ci" 'python3 scripts/quality/check-dependency-integrity.py --browser-only'
require "$ci" 'run: ./scripts/ci/test-go.sh "$MODULE"'
require "$ci" 'KINOSAIL_GO_TEST_MODE: quick'
require "$ci" 'go -C "$directory" run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...'
require "$ci" 'run: make installer-test native-build local-pipeline-test'
require "$ci" 'run: make -C apps/player client-check'
require "$ci" 'KINOSAIL_BROWSER_SMOKE: "1"'
require "$ci" 'KINOSAIL_BROWSER_PHASE: ${{ matrix.phase }}'
require "$ci" 'runs-on: ubuntu-24.04-arm'
require "$ci" 'actions/deploy-pages@'
require "$ci" "needs.plan.outputs.verified != 'true'"
grep -q '^  schedule:' "$ci" && fail 'ci.yml must leave scheduled deep runs to deep.yml'

require "$deep" '  schedule:'
require "$deep" 'CI_DEEP: "true"'
require "$deep" 'KINOSAIL_GO_TEST_MODE: deep'
require "$deep" 'KINOSAIL_BROWSER_MATRIX: full'
require "$deep" 'engine: [chromium, firefox, webkit]'
require "$deep" 'languages: swift'
require "$deep" './scripts/quality/check-static.sh'
require "$deep" 'for contract in iOS-backup iOS tvOS'
grep -q 'KINOSAIL_BROWSER_SMOKE' "$deep" && fail 'deep.yml must run complete browser suites'

require "$publish" '  workflow_call:'
require "$publish" 'run: python3 scripts/ci/delivery.py'
require "$publish" 'CI_RESULTS: ${{ inputs.results }}'
require "$publish" 'run: ./scripts/ci/test-image.sh "$APP" "$IMAGE"'
require "$publish" 'queue: max'
require "$publish" 'name: Advance ${{ matrix.app }} production tags'
require "$publish" 'cosign sign --yes'
require "$publish" 'COSIGN_REPOSITORY: ghcr.io/kinosail/kinosail-signatures'
require "$publish" 'push-to-registry: false'
require "$publish" 'python3 scripts/ci/promote.py'

require "$release" 'tags: ["player-v*", "subtitles-v*"]'
require "$release" 'run: python3 scripts/ci/release_tag.py'
require "$release" '--workflow ci.yml --commit "$commit" --event push'
require "$release" 'run: ./scripts/ci/package-release.sh "$APP" "$APP_VERSION"'
require "$release" 'cosign sign --yes "$IMAGE@$DIGEST"'
require "$release" 'COSIGN_REPOSITORY: ghcr.io/kinosail/kinosail-signatures'
require "$release" 'push-to-registry: false'
require "$release" 'gh release create "$RELEASE_TAG"'

require "$workflows/layout-stability.yml" 'contents: read'
require "$workflows/layout-stability.yml" 'runs-on: ubuntu-24.04'
require "$workflows/layout-stability.yml" 'run: python3 scripts/testing/test-layout-stability-local.py'
require "$workflows/layout-stability.yml" "contains(github.event.pull_request.labels.*.name, 'layout-evidence')"
if grep -Eiq 'campaign|witness|counterfactual|branches: \[codex/|(^|[^[:alnum:]])pr[0-9]+' "$workflows"/*.yml; then
  fail 'workflows must run durable suites without campaign, witness, or task-branch triggers'
fi
printf 'validated CI, deep, publication, version-release, and layout-evidence workflows\n'
