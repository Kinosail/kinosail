#!/usr/bin/env bash

prepare_audio_queue_fixture() {
  local repo
  repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  python3 "$repo/scripts/testing/test-player-audio-queue-local.py" --fixture-only "$1/R08 Fictional Session"
  chmod -R a+rX "$1/R08 Fictional Session"
}

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

run_populated_player_journeys() {
  local repo
  repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  KINOSAIL_BROWSER_PROJECT="$1" python3 "$repo/scripts/ci/run-populated-settings.py" \
    --url "$2" --output "$3" \
    --required-title 'settings search crosses levels and preserves unsaved preferences' \
    --required-title 'Owner settings search finds a setting across task families' \
    --required-title 'real Server rejects invalid progress without changing stored state and web reports the rejection' \
    --required-title 'populated player retries the latest progress through the real Server and renders accessible states' \
    --required-title 'real album queue advances source and all Now Playing identity to the fictional second track' \
    --required-title 'real album queue keeps system previous and next current and exposes only fresh current-track actions' \
    --required-title 'mobile R03 progress notice stays hidden after real acknowledgement and reopens only on failure' \
    -- pnpm --dir e2e test settings-discovery.spec.ts layout-audit-shell.spec.ts test-instance-progress.spec.ts test-instance-audio-queue.spec.ts --grep=@smoke --workers=1
}
