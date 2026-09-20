#!/usr/bin/env bash
# shellcheck source=scripts/tooling/gates-pause.sh
source "$(dirname "${BASH_SOURCE[0]}")/../tooling/gates-pause.sh"
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
readonly repo
readonly workflows="$repo/.github/workflows"
readonly dependabot="$repo/.github/dependabot.yml"
readonly pull_request_template="$repo/.github/pull_request_template.md"
readonly apps=(player subtitles dashboard)
release_images=()

fail() {
  printf 'workflow validation failed: %s\n' "$*" >&2
  exit 1
}

require_text() {
  local file="$1"
  local text="$2"
  grep -Fq -- "$text" "$file" || fail "$(basename "$file") must contain $text"
}

require_twice() {
  local file="$1"
  local text="$2"
  [[ "$(grep -Fxc -- "$text" "$file")" -eq 2 ]] ||
    fail "$(basename "$file") needs push and pull request entries for $text"
}

require_once() {
  local file="$1"
  local text="$2"
  [[ "$(grep -Fxc -- "$text" "$file")" -eq 1 ]] ||
    fail "$(basename "$file") must contain exactly one $text"
}

require_at_least() {
  local file="$1"
  local count="$2"
  local text="$3"
  [[ "$(grep -Fc -- "$text" "$file")" -ge "$count" ]] ||
    fail "$(basename "$file") must contain at least $count occurrences of $text"
}

[[ -f "$dependabot" ]] || fail "missing root .github/dependabot.yml"
require_once "$dependabot" 'version: 2'
require_once "$dependabot" '    directory: /packages'
require_once "$dependabot" '    directory: /apps/player'
require_once "$dependabot" '    directory: /apps/subtitles'
require_once "$dependabot" '    directory: /apps/dashboard'
require_once "$dependabot" '    directory: /apps/player/e2e'
require_once "$dependabot" '    directory: /scripts/quality'
require_once "$dependabot" '    directory: /apps/subtitles/e2e'
require_once "$dependabot" '  - package-ecosystem: github-actions'
[[ -f "$pull_request_template" ]] || fail "missing root pull request template"
require_text "$pull_request_template" '## Verification'

for app in "${apps[@]}"; do
  workflow="$workflows/$app-hygiene.yml"
  [[ -f "$workflow" ]] || fail "missing $app-hygiene.yml"

  require_text "$workflow" "working-directory: apps/$app"
  require_text "$workflow" "go-version-file: apps/$app/go.mod"
  require_text "$workflow" 'context: .'
  require_text "$workflow" "file: apps/$app/Containerfile"
  require_text "$workflow" "tags: localhost/kinosail-$app:\${{ github.sha }}"
  require_text "$workflow" "pull-requests: read"
  # shellcheck disable=SC2016 # GitHub expression must remain literal.
  require_text "$workflow" '--new-from-rev=${{'

  if grep -Eq '(^|[[:space:]])push:[[:space:]]+true([[:space:]]|$)' "$workflow"; then
    fail "$app-hygiene.yml must not publish containers"
  fi

  if grep -Fq 'localhost/kinosail:' "$workflow" || grep -Fq 'ghcr.io/' "$workflow"; then
    fail "$app-hygiene.yml uses an ambiguous or published container tag"
  fi

  release="$workflows/$app-release.yml"
  [[ -f "$release" ]] || fail "missing $app-release.yml"
  [[ ! -e "$repo/apps/$app/.github/workflows/release.yml" ]] ||
    fail "apps/$app contains an inert nested release workflow"

  require_text "$workflow" 'working-directory: packages'
  require_text "$workflow" 'run: make tidy-check coverage test-race'
  case "$app" in
    player|subtitles)
      require_text "$workflow" '  schedule:'
      require_text "$workflow" '    - cron: "'
      require_text "$workflow" '  scheduled:'
      require_text "$workflow" "name: $([[ "$app" == player ]] && printf Player || printf Subtitles) scheduled \${{ matrix.check }}"
      require_text "$workflow" "working-directory: apps/$app/e2e"
      require_text "$workflow" '        check: [performance, fuzz, deadcode]'
      require_text "$workflow" '            runner: ubuntu-24.04-arm'
      require_text "$workflow" '          KINOSAIL_BROWSER_MATRIX: full'
      require_text "$workflow" "          KINOSAIL_TEST_IMAGE: localhost/kinosail-$app:\${{ github.sha }}"
      require_text "$workflow" '      - run: make installer-test native-build local-pipeline-test'
      require_at_least "$workflow" 3 'sudo apt-get install -y libarchive-tools'
      [[ ! -e "$repo/apps/$app/.github/dependabot.yml" ]] ||
        fail "apps/$app contains an inert nested Dependabot configuration"
      [[ ! -e "$repo/apps/$app/.github/pull_request_template.md" ]] ||
        fail "apps/$app contains an inert nested pull request template"
      if [[ "$app" == player ]]; then
        require_text "$workflow" '  client:'
        require_text "$workflow" '      - run: make client-check'
        title=Player
      else
        title=Subtitles
      fi
      ;;
    dashboard) title=Dashboard ;;
  esac
  require_text "$release" "name: $title Release"
  require_text "$release" "tags: [\"$app-v*\"]"
  require_text "$release" "group: $app-release-\${{ github.ref }}"
  require_text "$release" "working-directory: apps/$app"
  require_text "$release" "version=\"\${GITHUB_REF_NAME#$app-}\""
  require_text "$release" "--workflow $app-hygiene.yml"
  require_text "$release" 'context: .'
  require_text "$release" "file: apps/$app/Containerfile"
  require_text "$release" "images: ghcr.io/kinosail/kinosail-$app"
  require_text "$release" "image-ref: ghcr.io/kinosail/kinosail-$app@\${{ steps.build.outputs.digest }}"
  require_text "$release" "subject-name: ghcr.io/kinosail/kinosail-$app"
  require_text "$release" "cosign sign --yes \"ghcr.io/kinosail/kinosail-$app@\$IMAGE_DIGEST\""
  require_text "$release" "cache-from: type=gha,scope=$app-release-container"
  require_text "$release" "cache-to: type=gha,mode=max,scope=$app-release-container"
  require_text "$release" '          sbom: true'
  require_text "$release" '          provenance: mode=max'
  require_text "$release" '      attestations: write'
  require_text "$release" '      id-token: write'
  require_text "$release" '      packages: write'
  require_text "$release" "gh release create \"\$RELEASE_TAG\""

  if grep -Fq 'tags: ["v*"]' "$release"; then
    fail "$app-release.yml accepts an unscoped release tag"
  fi
  if grep -Eq 'ghcr\.io/kinosail/kinosail([:@[:space:]]|$)' "$release"; then
    fail "$app-release.yml uses the ambiguous legacy Player image"
  fi

  image="ghcr.io/kinosail/kinosail-$app"
  for existing in "${release_images[@]:-}"; do
    [[ "$image" != "$existing" ]] || fail "$app-release.yml reuses image $image"
  done
  release_images+=("$image")

  case "$app" in
    player|subtitles)
      require_text "$release" "kinosail-$app-install.tar.gz"
      require_text "$release" "./scripts/package-native-release.sh \"\$APP_VERSION\" native-release"
      require_text "$release" 'native-release/kinosail-release.json.sigstore.json'
      require_text "$release" "subject-path: apps/$app/kinosail-$app-install.tar.gz"
      ;;
    dashboard)
      if grep -Fq 'package-native-release.sh' "$release" || grep -Fq 'install.tar.gz' "$release"; then
        fail "$app-release.yml references packaging the app does not provide"
      fi
      ;;
  esac
done

printf 'validated %s app hygiene and release workflow boundaries\n' "${#apps[@]}"
