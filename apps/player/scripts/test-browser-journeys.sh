#!/usr/bin/env bash

prepare_player_checkpoint_fixture() {
  local engine="$1" image="$2" media_dir="$3"
  shift 3
  "$engine" "$@" --rm --entrypoint sh "$image" -c 'ffmpeg -hide_banner -loglevel error -f lavfi -i testsrc2=size=320x180:rate=24:duration=30 -c:v libx264 -threads 1 -preset ultrafast -crf 35 -pix_fmt yuv420p -movflags +faststart /tmp/checkpoint.mp4 && cat /tmp/checkpoint.mp4' >"$media_dir/Checkpoint Example.mp4"
  chmod a+r "$media_dir/Checkpoint Example.mp4"
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
    --required-title 'completed paused seek persists before Library navigation and resumes actual movie frames' \
    --required-title 'Library exit checkpoints actual playing time before teardown without reset-position overwrite' \
    -- pnpm --dir e2e test settings-discovery.spec.ts layout-audit-shell.spec.ts test-instance-progress.spec.ts test-instance-checkpoint.spec.ts --grep=@smoke --workers=1
}
