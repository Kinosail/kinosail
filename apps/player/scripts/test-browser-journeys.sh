#!/usr/bin/env bash

run_library_pagination_journey() {
  GOMAXPROCS=2 KINOSAIL_LIBRARY_BROWSER=1 KINOSAIL_BROWSER_PROJECT="$1" \
    KINOSAIL_E2E_OUTPUT_DIR="$2" KINOSAIL_E2E_ARTIFACT_DIR="$3" \
    ../../scripts/tooling/with-go-module.sh go test -p 1 ./internal/server -run '^TestLibraryPaginationBrowserJourney$' -count=1 -timeout=6m
}

run_subtitle_recovery_journey() {
  GOMAXPROCS=2 KINOSAIL_CAPTION_BROWSER=1 KINOSAIL_BROWSER_PROJECT="$1" \
    KINOSAIL_CAPTION_MEDIA_FIXTURE="$2" KINOSAIL_E2E_OUTPUT_DIR="$3" KINOSAIL_E2E_ARTIFACT_DIR="$4" \
    ../../scripts/tooling/with-go-module.sh go test -p 1 ./internal/server -run '^TestSubtitleRecoveryBrowserJourney$' -count=1 -timeout=6m
}

run_download_pause_journeys() {
  local mode hit
  for mode in downloads-pause downloads-hit-targets; do
    hit=0
    [[ "$mode" == downloads-hit-targets ]] && hit=1
    GOMAXPROCS=2 KINOSAIL_DOWNLOAD_PAUSE_BROWSER=1 KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS="$hit" \
      KINOSAIL_BROWSER_PROJECT="$1" KINOSAIL_E2E_OUTPUT_DIR="$2-$mode" KINOSAIL_E2E_ARTIFACT_DIR="$3/$mode" \
      ../../scripts/tooling/with-go-module.sh go test -p 1 ./internal/server -run '^TestDownloadPauseBrowserJourney$' -count=1 -timeout=6m
  done
}

run_populated_player_journeys() {
  local repo
  repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  KINOSAIL_BROWSER_PROJECT="$1" python3 "$repo/scripts/ci/run-populated-settings.py" \
    --url "$2" --output "$3" \
    --required-title 'settings search crosses levels and preserves unsaved preferences' \
    --required-title 'Owner settings search finds a setting across task families' \
    --required-title 'real Server rejects invalid progress without changing stored state and web reports the rejection' \
    --required-title 'populated player retries the latest progress through the real Server and renders accessible states' \
    -- pnpm --dir e2e test settings-discovery.spec.ts layout-audit-shell.spec.ts test-instance-progress.spec.ts --grep=@smoke --workers=1
}
