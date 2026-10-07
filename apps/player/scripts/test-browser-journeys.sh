#!/usr/bin/env bash

# The container owner shares these volume names and the fixture directory.
# shellcheck disable=SC2034,SC2154
start_fresh_server() {
  local fixed_port="$1"
  if [[ -n "$container" ]]; then
    "$engine" rm --force "$container" >/dev/null
    container=""
  fi
  remove_state_volumes
  suffix="$$-$RANDOM"
  config_volume="kinosail-test-config-$suffix"
  cache_volume="kinosail-test-cache-$suffix"
  backup_volume="kinosail-test-backups-$suffix"
  create_state_volumes
  start_server "$fixed_port"
  if browser_fixture_uses_tls; then
    remove_browser_fixture_trust
    trust_browser_fixture_tls "$engine" "$container" "$mcp_dir" "$suffix"
  fi
}

prepare_player_checkpoint_fixture() {
  local engine="$1" image="$2" media_dir="$3"
  shift 3
  "$engine" "$@" --rm --entrypoint sh "$image" -c 'ffmpeg -hide_banner -loglevel error -f lavfi -i testsrc2=size=320x180:rate=24:duration=30 -c:v libx264 -threads 1 -preset ultrafast -crf 35 -pix_fmt yuv420p -movflags +faststart /tmp/checkpoint.mp4 && cat /tmp/checkpoint.mp4' >"$media_dir/Checkpoint Example.mp4"
  chmod a+r "$media_dir/Checkpoint Example.mp4"
}

prepare_audio_queue_fixture() {
  local repo
  repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
  python3 "$repo/scripts/testing/prepare-audio-queue-fixture.py" "$1/R08 Fictional Session"
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

run_native_intent_regression() {
  KINOSAIL_BROWSER_PROJECT="$1" KINOSAIL_E2E_VIDEO=off \
    KINOSAIL_E2E_OUTPUT_DIR="$2" KINOSAIL_E2E_ARTIFACT_DIR="$3" \
    PLAYWRIGHT_HTML_OUTPUT_DIR="$3/html-$1" \
    pnpm --dir e2e test player-native-intent-regression.spec.ts --workers=1 --retries=0
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
    --required-title 'volume icon renders balanced sound waves and keeps accessible mute controls' \
    --required-title 'real album queue advances source and all Now Playing identity to the fictional second track' \
    --required-title 'real album queue keeps system previous and next current and exposes only fresh current-track actions' \
    --required-title 'mobile R03 progress notice stays hidden after real acknowledgement and reopens only on failure' \
    --required-title 'album queue keeps accessible responsive controls through pending loaded empty and failed reads' \
    --required-title 'late initial queue response cannot warm media or publish controls after pagehide' \
    --required-title 'queued short track resumes its saved position without claiming unplayed progress' \
    --required-title 'ended offline queue requires its own watched acknowledgement: failed' \
    -- pnpm --dir e2e test settings-discovery.spec.ts layout-audit-shell.spec.ts test-instance-progress.spec.ts test-instance-checkpoint.spec.ts test-instance-volume-icon.spec.ts test-instance-audio-queue.spec.ts player-audio-policy.spec.ts player-audio-queue-lifecycle.spec.ts --grep=@smoke --workers=1
}
