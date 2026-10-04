#!/usr/bin/env bash
# Build real app binaries and keep the existing checksummed E2E receipt format.
set -euo pipefail
app="${1-all}"
if (( $# > 1 )) || [[ "$app" != all && "$app" != player && "$app" != subtitles ]]; then
  printf 'usage: %s [all|player|subtitles]\n' "$0" >&2
  exit 2
fi
repo="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repo/scripts/e2e"
export E2E_TELEMETRY_DISABLED=1
mkdir -p .e2e/bin
binaries="$(mktemp -d "$PWD/.e2e/bin/run.XXXXXX")"
trap 'rm -rf -- "$binaries"' EXIT
selected=(player subtitles)
[[ "$app" == all ]] || selected=("$app")
for product in "${selected[@]}"; do
  (cd "$repo" && go build -o "$binaries/$product" "./apps/$product/cmd/kinosail")
done
export KINOSAIL_E2E_PLAYER_BINARY="$binaries/player"
export KINOSAIL_E2E_SUBTITLES_BINARY="$binaries/subtitles"
run="$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -p ".e2e/runs/$run"
shasum -a 256 package.json pnpm-lock.yaml e2e.config.ts fixture.mjs tests/*.ts > ".e2e/runs/$run/inputs.sha256"
arguments=(run)
[[ "$app" == all ]] || arguments+=(--target "$app")
python3 ../ci/e2e-artifact.py --output ".e2e/runs/$run/context" -- \
  pnpm exec e2e "${arguments[@]}" --output ".e2e/runs/$run/runner"
