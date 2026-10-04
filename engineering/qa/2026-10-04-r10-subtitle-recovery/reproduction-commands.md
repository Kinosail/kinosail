# R10 prepared reproduction commands

Status: prepared; no runtime result yet. No production edits have been made.
Run only in the integration owner's serial browser/build slot.

Initial real-HTTP transport controls, from `apps/player/e2e`, twice with distinct
artifact directories. This is isolated caption-loader proof, not Go Server E2E:

```sh
KINOSAIL_CAPTION_ISOLATED=1 KINOSAIL_E2E_VIDEO=off \
  KINOSAIL_E2E_OUTPUT_DIR=/absolute/task/evidence/isolated-red-1 \
  KINOSAIL_E2E_ARTIFACT_DIR=/absolute/task/evidence/isolated-red-report-1 \
  pnpm exec playwright test player-subtitles-recovery.spec.ts \
  --workers=1 --project=chromium
```

Use `isolated-red-2` and `isolated-red-report-2` for the second independent run.
The proposed contract is failure and direct keyboard Retry after one 20-second
attempt deadline. The current caption loader should still show Loading
subtitles after the browser clock advances 20.1 seconds. Both delayed headers
and an open body must reproduce for the intended reason before product edits.

Populated real Go watch/render/bundle/media/caption transport, from `apps/player`:

```sh
GOMAXPROCS=2 KINOSAIL_CAPTION_BROWSER=1 KINOSAIL_E2E_VIDEO=off \
  KINOSAIL_CAPTION_MEDIA_FIXTURE='/absolute/task/R03 Example.mp4' \
  KINOSAIL_E2E_OUTPUT_DIR=/absolute/task/evidence/server-red-1 \
  KINOSAIL_E2E_ARTIFACT_DIR=/absolute/task/evidence/server-red-report-1 \
  ../../scripts/tooling/with-go-module.sh go test -v -p 1 ./internal/server \
  -run '^TestSubtitleRecoveryBrowserJourney$' -count=1 -timeout=6m
```

The supplied R03 fixture is fictional, 12 seconds, 320x180. Copy it read-only
into `t.TempDir`; preserve the original. Its SHA-256 is recorded in the failure
analysis and the Go runner logs. No encoder or probe process runs. The fixture
sets explicit unavailable FFmpeg/FFprobe paths and chooses direct playback.
Injected transport faults use a test-only HTTP wrapper; ordinary recovery gets
real sidecar bytes through the real Server. The runner has no production seam.
The optional isolated mode exercises the same regression contract without a
Go build; hosted selection will require the populated runner explicitly.

Required hosted hook changes remain with the integration owner while R03 owns
`test-container.sh`. A populated result requires six cases at 390, 1440 and
1920px, with real video time advancing, actual cues, source/choice continuity,
and pending/failed/loaded/Off screenshots. Do not interpret skipped ordinary
browser cases as populated proof.

Preparation checks: source cap and delta whitespace pass. The repository's
existing TypeScript type scan fails on pre-existing any/unknown cases and one
R04-owned new fixture assertion; root was notified to repair that assertion in
the frozen R04 integration checkout. New R10 tests contain neither forbidden
type. No type contract was weakened.
