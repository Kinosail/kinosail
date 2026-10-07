#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
# shellcheck disable=SC2016 # Assertions intentionally match literal Actions and shell expressions.
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
workflows="$repo/.github/workflows"
fail() { printf 'workflow validation failed: %s\n' "$*" >&2; exit 1; }
require() { grep -Fq -- "$2" "$1" || fail "$(basename "$1") must contain $2"; }

for name in ci app publish release layout-stability pr503-cold-publication-witness; do
  [[ -f "$workflows/$name.yml" ]] || fail "missing $name.yml"
done
[[ "$(find "$workflows" -maxdepth 1 -name '*.yml' -type f | wc -l | tr -d ' ')" == 6 ]] ||
  fail 'unexpected workflow file'
[[ -f "$repo/.github/dependabot.yml" ]] || fail 'missing Dependabot configuration'
[[ -f "$repo/.github/pull_request_template.md" ]] || fail 'missing pull request template'

ci="$workflows/ci.yml"
app="$workflows/app.yml"
publish="$workflows/publish.yml"
release="$workflows/release.yml"
require "$ci" '  pull_request:'
require "$ci" '    branches: [main]'
require "$ci" '  merge_group:'
require "$ci" 'run: python3 scripts/ci/affected.py'
require "$ci" 'run: python3 scripts/ci/required.py repository'
require "$ci" 'run: python3 scripts/ci/required.py security'
require "$ci" 'results: ${{ toJSON(needs) }}'
require "$ci" 'uses: ./.github/workflows/app.yml'
require "$ci" 'uses: ./.github/workflows/publish.yml'
require "$ci" 'python3 scripts/quality/check-dependency-integrity.py --browser-only'
require "$ci" './scripts/ci/test-go.sh packages'
require "$ci" 'actions/deploy-pages@'

require "$app" '  workflow_call:'
require "$app" 'if: fromJSON(inputs.plan)[inputs.app]'
require "$app" 'run: ./scripts/ci/test-go.sh "$APP"'
require "$app" 'run: go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...'
require "$app" 'run: make installer-test native-build local-pipeline-test'
require "$app" 'run: make -C apps/player client-check'
require "$app" 'run: python3 scripts/ci/required.py app'
require "$app" '"runner":"ubuntu-24.04-arm"'
require "$app" 'KINOSAIL_BROWSER_SMOKE:'

require "$publish" '  workflow_call:'
require "$publish" 'run: python3 scripts/ci/delivery.py'
require "$publish" 'CI_RESULTS: ${{ inputs.results }}'
require "$publish" 'run: ./scripts/ci/test-image.sh "$APP" "$IMAGE"'
require "$publish" 'queue: max'
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
witness="$workflows/pr503-cold-publication-witness.yml"
require "$witness" '  contents: read'
require "$witness" '    shell: bash'
require "$witness" '    runs-on: ubuntu-24.04'
require "$witness" '    timeout-minutes: 10'
require "$witness" '          ref: ${{ github.event.pull_request.head.sha }}'
require "$witness" 'go test -json -count=100 -run '\''^TestCopiedRecoveryEnclosingOrdinaryPublication$'\'''
require "$witness" 'go test -json -count=10 -run '\''^TestCopiedRecoveryEnclosing('
require "$witness" 'go test -json -count=10 -run '\''^TestCopiedRecoveryOrdinaryPublicationInitializationCounterfactual$'\'''
require "$witness" 'go test -json -count=10 -run '\''^TestCopiedRecoveryEncoderInitializationRemainsCommittedUntilReplacement$'\'''
require "$witness" '          retention-days: 14'
require "$witness" '          if-no-files-found: error'
if grep -Eq 'continue-on-error|\|\|[[:space:]]*true|contents:[[:space:]]*write' "$witness"; then
  fail 'cold publication witness must preserve failures and read-only permissions'
fi
printf 'validated CI, app, publication, version-release, layout-evidence, and cold-publication witness workflows\n'
