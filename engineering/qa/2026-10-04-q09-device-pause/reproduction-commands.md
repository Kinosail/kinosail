# Q09 repeatable commands

Run from the owned `codex/qol-q09-device-pause` checkout. Root grants each
serial browser/build slot. The initial run is documented in `report.md`;
clean completed repeats and the separately selected controls remain pending.

## Expected baseline absence, two independent native HTTP runs

From `apps/player/e2e`, set a different output directory for each repeat:

```sh
PLAYWRIGHT_CHANNEL="" KINOSAIL_DOWNLOAD_PAUSE_ISOLATED=1 KINOSAIL_BROWSER_WORKERS=1 \
KINOSAIL_E2E_VIDEO=off \
KINOSAIL_E2E_OUTPUT_DIR="$PWD/../../../engineering/qa/2026-10-04-q09-device-pause/isolated-red-2" \
KINOSAIL_E2E_ARTIFACT_DIR="$PWD/../../../engineering/qa/2026-10-04-q09-device-pause/isolated-red-2-report" \
pnpm exec playwright test download-pause.spec.ts --workers=1 --project=chromium
```

Each run executes two acceptance cases: actual OPFS at 390 pixels and forced
native IndexedDB fallback at 1440 pixels. Expected baseline boundary: an intact
8 MiB first block and a held partial second range, followed by missing Pause.
An unrelated harness error is not acceptance RED evidence.

An empty `PLAYWRIGHT_CHANNEL` selects Playwright's cached Chromium rather than
the macOS default installed Chrome channel. Each executed case attaches the
runtime browser/version/channel. The run context must also record the executable
path/hash and exact committed source snapshot. Give each primary command a
40-second external bound and the separately selected profile controls a
35-second bound. A timed-out command is incomplete, even when a case trace
contains the intended assertion. Do not apply production changes until both
completed primary repeats and the dedicated profile controls are preserved.

Use the same environment/output overrides for the separately repeated same-profile
control, selecting only its title:

```sh
pnpm exec playwright test download-pause-ownership.spec.ts --workers=1 --project=chromium \
  --grep 'same Viewer Profile'
```

Select the changed-profile negative control with `--grep 'changing Viewer Profile'`.
This case uses an isolated supplied profile; it does not create or change accounts.

## Go-rendered/native storage journey

From `apps/player`, with a root-granted build slot:

```sh
GOMAXPROCS=2 PLAYWRIGHT_CHANNEL="" KINOSAIL_DOWNLOAD_PAUSE_BROWSER=1 KINOSAIL_E2E_VIDEO=off \
KINOSAIL_E2E_OUTPUT_DIR="$PWD/../../engineering/qa/2026-10-04-q09-device-pause/server-green" \
KINOSAIL_E2E_ARTIFACT_DIR="$PWD/../../engineering/qa/2026-10-04-q09-device-pause/server-green-report" \
../../scripts/tooling/with-go-module.sh go test -v -p 1 ./internal/server \
  -run '^TestDownloadPauseBrowserJourney$' -count=1 -timeout=6m
```

The fixture prepares fictional deterministic bytes through the public API,
original quality, in temporary Server media/data/cache directories. It never
runs an encoder or exercises Remove. Six cases cover OPFS and IndexedDB at
390, 1440 and 1920 pixels, with native service-worker and lock admission. Two
additional real Server cases cover same-profile tab ownership and actual
navigation/reload. The isolated supplied-profile control is not registered in
the Go run, so the populated suite executes eight cases with no deliberate skip.

Required hosted/container gate wiring remains root-owned. The runner is opt-in
for ordinary Go invocation; an ordinary skipped Go test is not browser proof.

## Additional real Go phone hit-target control

After a root-granted serial slot, use the same actual Go command with
`KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS=1` and distinct `server-hit-targets` output
and report directories. This selects only `download-pause-hit-target.spec.ts`:
one actual Go phone case for centered pointer Pause/Resume and pointer/keyboard
Play navigation. The default mode still selects the original eight cases.
Record the exact selected source revision and completed nonzero case count.
The fixture is fictional and non-playable; this establishes no media decoding.

Completed repeat: source `0606ac620a2a8348e9fbf55a239aeca11d28688f`, unchanged
product `bf10089e`, directories `server-hit-targets-stable` and
`server-hit-targets-stable-report`. Read `server-hit-targets-stable-receipt.json`
for the exact command/env, 60-second external bound and complete nonzero result.
The test retains native smooth scrolling, observes three equal geometry/scroll
samples within two seconds, then requires every original hit/focus/navigation
assertion. The earlier timing-boundary run and the default eight-case Go proof
remain separately preserved; this flag does not alter that default selection.
