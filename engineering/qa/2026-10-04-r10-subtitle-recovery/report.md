# R10: stalled subtitle deadline and Retry

Status: tested in the isolated browser. Deadline runtime RED is confirmed
twice, and the separate caption restore RED is repeated twice. The affected
isolated suite passed 12 / 12 at `e3651e1e`, with no skipped, failed or flaky
cases and no errors, in 12.0 seconds. The real Go-rendered playable-video journey also passed all six cases at
`6af39f2f`: 0 skipped, unexpected, flaky or errors; browser 12.1 seconds and
Go package 19.231 seconds. No merge or deployment is claimed.

The selected caption could remain at “Loading subtitles…” indefinitely when
headers or a VTT body stopped arriving. Both independent real-HTTP isolated
runs reproduced both stalls at unchanged revision
`1e3e47f5ddc27422c3c6ebede5b271a06429d796`: four intended failures, without a
setup or harness failure. The browser clock advanced 20.1 seconds; this is a
deterministic timer proof, not a measured 20-second wall-time test or incident
frequency estimate.

The deadline repair is `52038089`. A caption attempt has one 20-second deadline
through response headers, streamed bytes and native track loading. A failed
attempt cancels its transport, removes stale listeners/timer, removes and
revokes its failed Blob source, then exposes Retry. Retry preserves the track,
its mode and video state and returns keyboard focus to the subtitle selector.
Status belongs to the current showing track. Diagnostics use fixed failure
classes and a validated bounded request ID; notices contain no remote URL or
error text. Existing origin, redirect, MIME, empty and 16 MiB limits remain.

The Go template adds a quiet Retry button outside the subtitle label and gives
the live status a stable accessible ID. The effective player asset token
already derives from its composed content, so the changed asset receives a new
immutable token without an unrelated locale bump. Shared progress, Now Playing,
Android, Apple, deployments and PiP/miniplayer are unchanged.

## Evidence and repeatability

- [Failure analysis](failure-analysis.md) precedes production changes.
- [Commands](reproduction-commands.md) record the isolated controls and pending
  real Server runner.
- [Isolated RED context](isolated-red-context.json) records exact source and
  fixture hashes, environment, results and coverage boundaries.
- Logs, traces, screenshots and JSON reports are retained in the two
  `isolated-red-*` directories. Compressed text uses deterministic gzip and was
  verified to decompress to the original bytes before compression.
- `product-browser-lint.log` and `lifecycle-browser-lint.log`: 0 errors and 0 warnings.
- `isolated-green-context.json`: exact tested product/test hashes and all 12 results.
- `server-green-context.json`: six real Server results, actual served bundle
  receipts, transport closure receipts, environment and media hash.
- `server-green.log.gz`: actual Go test and request/playback diagnostics.
- `server-green/`: 24 screenshots covering both stalls and all three widths.
  Twelve body-state screenshots were inspected in one responsive batch; see
  `visual-review.md`.
- `product-max-loc.log`: passed, with no output.
- `evidence-gitleaks.log`: no leaks found in retained evidence.
- `artifact-sha256.json`: current artifact hashes; refreshed at final handoff.
- Full TypeScript forbidden-type scanning remains red on pre-existing files in
  this preserved branch, including the frozen R04 fixture which the integration
  owner repaired separately. No R10 file is reported; full scan green is not claimed.

Baseline failures occur before cancellation and Retry assertions. The test
teardown closes only held task-owned responses. Product cancellation is not
claimed as baseline proof. The prepared GREEN assertions check actual peer
request closure, keyboard Retry, recovered native cues, no later false timeout,
caption preference, unchanged video source/pause state and continued playback
in the Go-rendered fixture.

## Caption lifecycle follow-up

`ce9b8caa` adds a caption-only persisted lifecycle control before its repair.
Both repetitions at `04997617` confirmed that the old request closes but the
selected caption does not reload after persisted pageshow. Exact baseline
artifacts are retained in `lifecycle-red*` and committed in `5eb53ad9` before
the repair. Product `e3651e1e` refreshes only the caption controller and attempt
map on persisted restore, then reloads the current showing track. It captures
the original lifecycle signal for cleanup and clears stale status on pagehide.
Both lifecycle cases pass; Off starts no new request. Native PageTransitionEvent
dispatch controls the caption lifecycle in one document; it does not establish
actual BFCache admission or browser navigation behavior.

## Remaining verification boundaries

The isolated shell runs the actual caption loader over a real local HTTP peer.
The completed `TestSubtitleRecoveryBrowserJourney` additionally copies the
authorized fictional 12-second R03 video into temporary media with synthetic
captions, renders through the actual Go Server, attests the served asset token
against its bytes and plays real video. All six token/byte receipts match
`0ff9a9da7c93d26ef3a756f46c9f98fb72a23bd08d75491a2873743e31579b51`.
Each deadline receipt observes one caption request and one closed request.
Recovered active cues appear over real advancing video. Off is preserved during
pending work and after recovery; source/pause state remains unchanged.
The original fixture is preserved. No probe, encoder or deployment executes.
The configured unavailable probes explain expected marker-duration/discovery
startup warnings; probe, marker and device availability are not established.
Actual BFCache admission/navigation, WebKit, Firefox and physical devices remain
unverified locally. Deadline timing uses deterministic browser clock control,
while video frames advance through native real-time decoding.

Independent source/evidence review is clear at product `e3651e1e`, with the
stronger Go-only harness rechecked at `7dfe6005`; see `independent-review.md`.
Required CI hook, `make verify-changed`, `make test-instance-check`, final
protected checks, PR, merge and fetched ancestry are owned by the integration
owner. Final populated evidence review is independently clear at `2ace6b75`: all
163 artifacts, eight source/context inputs, composed asset and responsive
captures match. This report update records that clearance without a product
or harness change. `test-container.sh` remains untouched during R03 integration.

MAIN: NO — awaiting required integration/hosted gates and merge ancestry.
